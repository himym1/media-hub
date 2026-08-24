package strm

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"media-hub/backend/internal/config"
	"media-hub/backend/internal/integration"
	"media-hub/backend/internal/store"
)

type SessionChecker interface {
	SessionUserID(context.Context) (string, error)
}

type Alerter interface {
	Configured() bool
	Send(context.Context, string) (bool, error)
}

type LibraryRefresher interface {
	RefreshLibrary(context.Context, string) error
}

type Coordinator struct {
	store     *store.Store
	syncer    Syncer
	session   SessionChecker
	redirect  *RedirectCache
	workflow  func() config.Workflow
	alerter   Alerter
	emby      LibraryRefresher
	logger    *slog.Logger
	now       func() time.Time
	mutex     sync.Mutex
	running   bool
	lastErr   string
	lastAlert time.Time
}

type Status struct {
	Mode          string `json:"mode"`
	Running       bool   `json:"running"`
	MountPath     string `json:"mountPath"`
	MountWritable bool   `json:"mountWritable"`
	SessionOK     bool   `json:"sessionOk"`
	LastError     string `json:"lastError,omitempty"`
	LastSummary   string `json:"lastSummary,omitempty"`
	LastSyncAt    int64  `json:"lastSyncAt,omitempty"`
	LastMediaType string `json:"lastMediaType,omitempty"`
}

type LibrarySyncInput struct {
	MediaType string
	Full      bool
	DryRun    bool
}

func NewCoordinator(dataStore *store.Store, syncer Syncer, session SessionChecker, redirect *RedirectCache, workflow func() config.Workflow, logger *slog.Logger) *Coordinator {
	if logger == nil {
		logger = slog.Default()
	}
	return &Coordinator{
		store: dataStore, syncer: syncer, session: session, redirect: redirect,
		workflow: workflow, logger: logger, now: time.Now,
	}
}

func (c *Coordinator) UseAlerter(alerter Alerter) {
	if c == nil {
		return
	}
	c.alerter = alerter
}

func (c *Coordinator) UseLibraryRefresher(refresher LibraryRefresher) {
	if c == nil {
		return
	}
	c.emby = refresher
}

func (c *Coordinator) Sync(ctx context.Context, req Request) (Result, error) {
	return c.SyncTransfer(ctx, req)
}

func (c *Coordinator) Check(ctx context.Context) integration.Health {
	health := integration.Health{ID: "strm", Label: "内置 STRM"}
	if c == nil || c.workflow == nil {
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未配置内置 STRM"
		return health
	}
	workflow := c.workflow()
	if !workflow.UsesBuiltinSync() {
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未配置内置 STRM"
		return health
	}
	if c.session == nil {
		health.Status = integration.StatusUnconfigured
		health.Detail = "115 会话未接入"
		return health
	}
	if _, err := c.session.SessionUserID(ctx); err != nil {
		health.Status = integration.StatusDegraded
		health.Detail = "115 会话不可用"
		return health
	}
	if err := MountWritable(workflow.StrmRootMount); err != nil {
		health.Status = integration.StatusDegraded
		health.Detail = "STRM 挂载不可写"
		return health
	}
	health.Status = integration.StatusHealthy
	health.Detail = "内置 STRM 可写，115 会话有效"
	c.mutex.Lock()
	lastErr := c.lastErr
	c.mutex.Unlock()
	if lastErr != "" {
		health.Status = integration.StatusDegraded
		health.Detail = "最近一次同步失败"
	}
	return health
}

func (c *Coordinator) Status(ctx context.Context) (Status, error) {
	status := Status{Mode: ModeBuiltin}
	if c == nil || c.workflow == nil {
		return status, nil
	}
	workflow := c.workflow()
	status.Mode = workflow.NormalizedSyncMode()
	status.MountPath = workflow.StrmRootMount
	status.MountWritable = MountWritable(workflow.StrmRootMount) == nil
	if c.session != nil {
		_, err := c.session.SessionUserID(ctx)
		status.SessionOK = err == nil
	}
	c.mutex.Lock()
	status.LastError = c.lastErr
	status.Running = c.running
	c.mutex.Unlock()
	if c.store != nil {
		if run, ok, err := c.store.LatestSTRMSyncRun(ctx); err == nil && ok {
			status.LastSyncAt = run.Finished
			status.LastMediaType = run.MediaType
			status.LastSummary = Result{
				Scanned: run.Scanned, Created: run.Created, Updated: run.Updated, Skipped: run.Skipped, Removed: run.Removed,
			}.Summary()
			if run.Error != "" {
				status.LastError = run.Error
			}
		}
	}
	return status, nil
}

func (c *Coordinator) Redirect(ctx context.Context, pickCode, name, userAgent string) (string, error) {
	if c == nil || c.redirect == nil {
		return "", ErrInvalidRequest
	}
	return c.redirect.Resolve(ctx, pickCode, name, userAgent)
}

func (c *Coordinator) SyncTransfer(ctx context.Context, req Request) (Result, error) {
	if c == nil || c.syncer == nil {
		return Result{}, ErrInvalidRequest
	}
	result, err := c.syncer.Sync(ctx, req)
	c.recordError(ctx, err)
	return result, err
}

func (c *Coordinator) EnqueueLibrarySync(input LibrarySyncInput) error {
	if c == nil || c.syncer == nil || c.workflow == nil || !c.workflow().UsesBuiltinSync() {
		return ErrInvalidRequest
	}
	if !c.tryStart() {
		return ErrBusy
	}
	go func() {
		defer c.finish()
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
		defer cancel()
		_, _ = c.syncLibrary(ctx, input)
	}()
	return nil
}

func (c *Coordinator) SyncLibrary(ctx context.Context, input LibrarySyncInput) (Result, error) {
	if c == nil || c.syncer == nil || c.workflow == nil {
		return Result{}, ErrInvalidRequest
	}
	if !c.workflow().UsesBuiltinSync() {
		return Result{}, ErrInvalidRequest
	}
	if !c.tryStart() {
		return Result{}, ErrBusy
	}
	defer c.finish()
	return c.syncLibrary(ctx, input)
}

func (c *Coordinator) syncLibrary(ctx context.Context, input LibrarySyncInput) (Result, error) {
	mediaType := strings.ToLower(strings.TrimSpace(input.MediaType))
	if mediaType == "" || mediaType == "all" {
		var combined Result
		var first error
		started := c.now()
		for _, item := range []string{"movie", "series"} {
			result, err := c.syncOneLibrary(ctx, LibrarySyncInput{MediaType: item, Full: input.Full, DryRun: input.DryRun})
			combined.Scanned += result.Scanned
			combined.Created += result.Created
			combined.Updated += result.Updated
			combined.Skipped += result.Skipped
			combined.Removed += result.Removed
			if err != nil && first == nil && !errors.Is(err, ErrNoVideos) {
				first = err
			}
		}
		combined.Duration = c.now().Sub(started)
		return combined, first
	}
	return c.syncOneLibrary(ctx, input)
}

func (c *Coordinator) syncOneLibrary(ctx context.Context, input LibrarySyncInput) (Result, error) {
	workflow := c.workflow()
	mediaType := strings.TrimSpace(input.MediaType)
	if mediaType == "" {
		mediaType = "movie"
	}
	target, ok := workflow.Target(mediaType)
	if !ok {
		return Result{}, ErrInvalidRequest
	}
	full := input.Full
	if c.store != nil && !full {
		lastFull, err := c.store.LatestFullSTRMSyncAt(ctx)
		if err == nil && (lastFull == 0 || c.now().UTC().Unix()-lastFull >= int64(FullSyncEvery.Seconds())) {
			full = true
		}
	}
	known := map[string]int64{}
	if !full && c.store != nil {
		if states, err := c.store.STRMFolderStates(ctx, target.QMediaSyncTargetPath); err == nil {
			known = states
		}
	}
	started := c.now().UTC().Unix()
	result, err := c.syncer.Sync(ctx, Request{
		FileID:          target.DestinationID,
		TargetPath:      target.QMediaSyncTargetPath,
		LibraryRoot:     true,
		Prune:           full,
		Incremental:     !full,
		DryRun:          input.DryRun,
		ContinueOnError: true,
		MinVideoSize:    DefaultMinVideoSize,
		KnownFolders:    known,
		StrmBaseURL:     workflow.StrmBaseURL,
		StrmRootMount:   workflow.StrmRootMount,
	})
	finished := c.now().UTC().Unix()
	c.recordError(ctx, err)
	if c.store != nil && !input.DryRun {
		message := ""
		if err != nil {
			message = publicError(err)
		}
		_ = c.store.InsertSTRMSyncRun(ctx, store.STRMSyncRun{
			MediaType: mediaType, Full: full, Scanned: result.Scanned, Created: result.Created,
			Updated: result.Updated, Skipped: result.Skipped, Removed: result.Removed,
			Error: message, StartedAt: started, Finished: finished,
		})
		if err == nil && len(result.Folders) > 0 {
			_ = c.store.ReplaceSTRMFolderStates(ctx, target.QMediaSyncTargetPath, result.Folders, !full)
		}
	}
	if err == nil && !input.DryRun && c.emby != nil && strings.TrimSpace(target.EmbyLibraryID) != "" {
		if refreshErr := c.emby.RefreshLibrary(ctx, target.EmbyLibraryID); refreshErr != nil {
			c.logger.Info("strm emby refresh failed", "mediaType", mediaType)
		}
	}
	c.logger.Info("strm library sync finished", "mediaType", mediaType, "full", full, "dryRun", input.DryRun, "created", result.Created, "updated", result.Updated, "removed", result.Removed, "failed", err != nil)
	return result, err
}

func (c *Coordinator) Start(ctx context.Context) {
	if c == nil {
		return
	}
	go c.run(ctx)
}

func (c *Coordinator) run(ctx context.Context) {
	timer := time.NewTimer(2 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			c.syncScheduled(ctx)
			timer.Reset(DefaultSyncEvery)
		}
	}
}

func (c *Coordinator) syncScheduled(ctx context.Context) {
	if c.workflow == nil || !c.workflow().UsesBuiltinSync() {
		return
	}
	if _, err := c.SyncLibrary(ctx, LibrarySyncInput{MediaType: "all"}); err != nil && !errors.Is(err, ErrNoVideos) && !errors.Is(err, ErrInvalidRequest) && !errors.Is(err, ErrBusy) {
		c.logger.Info("strm scheduled sync failed")
	}
}

func (c *Coordinator) tryStart() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.running {
		return false
	}
	c.running = true
	return true
}

func (c *Coordinator) finish() {
	c.mutex.Lock()
	c.running = false
	c.mutex.Unlock()
}

func (c *Coordinator) recordError(ctx context.Context, err error) {
	c.mutex.Lock()
	if err == nil || errors.Is(err, ErrNoVideos) {
		c.lastErr = ""
		c.mutex.Unlock()
		return
	}
	c.lastErr = publicError(err)
	c.mutex.Unlock()
	if errors.Is(err, ErrAuthExpired) {
		c.alertAuth(ctx)
	}
}

func (c *Coordinator) alertAuth(ctx context.Context) {
	if c.alerter == nil || !c.alerter.Configured() {
		return
	}
	now := c.now()
	c.mutex.Lock()
	if !c.lastAlert.IsZero() && now.Sub(c.lastAlert) < FullSyncEvery {
		c.mutex.Unlock()
		return
	}
	c.lastAlert = now
	c.mutex.Unlock()
	_, _ = c.alerter.Send(ctx, "Media Hub\n内置 STRM 的 115 会话已失效，请在概览页重新扫码")
}

func publicError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrAuthExpired):
		return "115 授权已失效"
	case errors.Is(err, ErrPathUnwritable):
		return "STRM 目录不可写"
	case errors.Is(err, ErrNoVideos):
		return "没有可同步的视频"
	case errors.Is(err, ErrBusy):
		return "已有同步任务在执行"
	default:
		return "STRM 同步失败"
	}
}
