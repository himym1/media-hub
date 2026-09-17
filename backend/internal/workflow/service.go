package workflow

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"sync"
	"time"

	"media-hub/backend/internal/adapter"
	"media-hub/backend/internal/config"
	"media-hub/backend/internal/emby"
	"media-hub/backend/internal/search"
	"media-hub/backend/internal/selection"
	"media-hub/backend/internal/settings"
	"media-hub/backend/internal/store"
	"media-hub/backend/internal/strm"
)

var (
	ErrUnavailable        = errors.New("transfer workflow is not configured")
	ErrInvalidSelection   = errors.New("selection token is invalid or expired")
	ErrSourceUnavailable  = errors.New("selected source does not support transfer")
	ErrTargetUnavailable  = errors.New("transfer target is not configured")
	ErrInvalidIdempotency = errors.New("invalid idempotency key")
)

var idempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,100}$`)

const (
	selectionLifetime = 15 * time.Minute
	jobTokenLifetime  = 30 * 24 * time.Hour
)

type Notifier interface {
	Configured() bool
	Send(context.Context, string) (bool, error)
}

type SourcePathResolver func(context.Context, string) (string, error)

// SourceRenamer renames a 115 file or folder before STRM sync so directories
// inherit a stable Title (Year) name instead of release watermarks.
type SourceRenamer func(context.Context, string, string) error

// TransferredContentValidator inspects a transferred 115 folder before STRM sync.
type TransferredContentValidator func(context.Context, string, string) error

// FolderVideoInspector reports whether a 115 folder already contains video files.
type FolderVideoInspector func(context.Context, string) (bool, error)

// SubtitleAttacher hangs a Chinese sidecar on an Emby movie or episode.
// Status is existed, attached, or missing; errors must not fail the transfer.
type SubtitleAttacher interface {
	AttachChinese(context.Context, string) (string, error)
}

type Service struct {
	store             *store.Store
	search            *search.Service
	codec             *selection.Codec
	emby              *emby.Client
	notifier          Notifier
	resolveSourcePath SourcePathResolver
	renameSource      SourceRenamer
	validateTransfer  TransferredContentValidator
	folderHasVideos   FolderVideoInspector
	subtitles         SubtitleAttacher
	strm              strm.Syncer
	mutex             sync.RWMutex
	workflow          config.Workflow
	now               func() time.Time
	wake              chan struct{}
	done              chan struct{}
}

func NewService(
	dataStore *store.Store,
	searchService *search.Service,
	codec *selection.Codec,
	embyClient *emby.Client,
	notifier Notifier,
	workflowConfig config.Workflow,
	resolveSourcePath SourcePathResolver,
	renameSource SourceRenamer,
	validateTransfer TransferredContentValidator,
) *Service {
	return &Service{
		store: dataStore, search: searchService, codec: codec, emby: embyClient,
		notifier: notifier, workflow: workflowConfig, resolveSourcePath: resolveSourcePath,
		renameSource: renameSource, validateTransfer: validateTransfer, now: time.Now,
		wake: make(chan struct{}, 1), done: make(chan struct{}),
	}
}

func (s *Service) Configure(workflowConfig config.Workflow) {
	s.mutex.Lock()
	s.workflow = workflowConfig
	s.mutex.Unlock()
}

func (s *Service) Workflow() config.Workflow {
	return s.workflowConfiguration()
}

func (s *Service) UseSTRMSyncer(syncer strm.Syncer) {
	s.mutex.Lock()
	s.strm = syncer
	s.mutex.Unlock()
}

func (s *Service) UseFolderVideos(inspect FolderVideoInspector) {
	s.mutex.Lock()
	s.folderHasVideos = inspect
	s.mutex.Unlock()
}

func (s *Service) UseSubtitles(attacher SubtitleAttacher) {
	s.mutex.Lock()
	s.subtitles = attacher
	s.mutex.Unlock()
}

func (s *Service) folderVideos() FolderVideoInspector {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.folderHasVideos
}

func (s *Service) subtitleAttacher() SubtitleAttacher {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.subtitles
}

func (s *Service) strmSyncer() strm.Syncer {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.strm
}

func (s *Service) syncConfigured() bool {
	if s.emby == nil || !s.emby.Configured() {
		return false
	}
	workflow := s.workflowConfiguration()
	if _, ok := workflow.Target("movie"); ok {
		return true
	}
	if _, ok := workflow.Target("series"); ok {
		return true
	}
	_, ok := workflow.Target("adult")
	return ok
}

func (s *Service) workflowConfiguration() config.Workflow {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.workflow
}

func (s *Service) SelectionToken(candidate search.Candidate) string {
	if s.codec == nil || candidate.SourceRef == "" {
		return ""
	}
	switch candidate.TransferState {
	case "downloadable":
		if _, ok := s.search.DownloadSource(candidate.SourceID); !ok {
			return ""
		}
	case "available":
		if !s.syncConfigured() {
			return ""
		}
		if _, ok := s.workflowConfiguration().Target(candidate.MediaType); !ok {
			return ""
		}
		if candidate.MediaType == "series" && (candidate.Season == 0 || candidate.EpisodeStart == 0 || candidate.EpisodeEnd == 0) {
			return ""
		}
		if _, ok := s.search.TransferSource(candidate.SourceID); !ok {
			return ""
		}
	default:
		return ""
	}
	if candidate.Revision == 0 || candidate.Revision != s.search.CurrentRevision() {
		return ""
	}
	token, err := s.codec.Encode(selection.Payload{
		SourceID: candidate.SourceID, CandidateID: candidate.ID, Title: candidate.Title,
		Year: candidate.Year, Season: candidate.Season, EpisodeStart: candidate.EpisodeStart, EpisodeEnd: candidate.EpisodeEnd,
		MediaType: candidate.MediaType,
		TMDBID:    candidate.TMDBID, Reference: candidate.SourceRef,
		ExpiresAt: s.now().UTC().Add(selectionLifetime).Unix(), Revision: candidate.Revision,
	})
	if err != nil {
		return ""
	}
	return token
}

func (s *Service) EnqueueShareImport(ctx context.Context, userID int64, title, rawURL, receiveCode, idempotencyKey string) (Job, bool, error) {
	parsed, err := adapter.ParseAdultImport(rawURL, receiveCode)
	if err != nil {
		return Job{}, false, ErrInvalidSelection
	}
	title = adapter.ImportDefaultTitle(title, parsed)
	if _, ok := s.workflowConfiguration().Target("adult"); !ok {
		return Job{}, false, ErrTargetUnavailable
	}
	if _, ok := s.search.TransferSource(adapter.ShareSourceID); !ok {
		return Job{}, false, ErrSourceUnavailable
	}
	var reference string
	if parsed.Kind == adapter.ImportKindShare {
		reference, err = adapter.ShareReferenceJSON(title, parsed.ShareCode, parsed.ReceiveCode)
	} else {
		reference, err = adapter.URLReferenceJSON(title, parsed.URL)
	}
	if err != nil {
		return Job{}, false, ErrInvalidSelection
	}
	token := s.SelectionToken(search.Candidate{
		ID: adapter.ImportCandidateID(parsed), Title: title, MediaType: "adult",
		SourceID: adapter.ShareSourceID, SourceRef: reference,
		TransferState: "available", Revision: s.search.CurrentRevision(),
	})
	if token == "" {
		return Job{}, false, ErrInvalidSelection
	}
	return s.Enqueue(ctx, userID, token, idempotencyKey)
}

func (s *Service) Enqueue(ctx context.Context, userID int64, selectionToken, idempotencyKey string) (Job, bool, error) {
	settings.ProviderSettingsLock.RLock()
	defer settings.ProviderSettingsLock.RUnlock()
	if s.codec == nil {
		return Job{}, false, ErrUnavailable
	}
	if !idempotencyPattern.MatchString(idempotencyKey) {
		return Job{}, false, ErrInvalidIdempotency
	}
	payload, err := s.codec.Decode(selectionToken)
	if err != nil {
		return Job{}, false, ErrInvalidSelection
	}
	if payload.Revision != s.search.CurrentRevision() {
		return Job{}, false, ErrInvalidSelection
	}
	if _, ok := s.search.DownloadSource(payload.SourceID); !ok {
		if !s.syncConfigured() {
			return Job{}, false, ErrTargetUnavailable
		}
		if _, ok := s.workflowConfiguration().Target(payload.MediaType); !ok {
			return Job{}, false, ErrTargetUnavailable
		}
		if _, ok := s.search.TransferSource(payload.SourceID); !ok {
			return Job{}, false, ErrSourceUnavailable
		}
	}

	canonical, err := json.Marshal(struct {
		SourceID     string `json:"sourceId"`
		CandidateID  string `json:"candidateId"`
		Season       int    `json:"season"`
		EpisodeStart int    `json:"episodeStart"`
		EpisodeEnd   int    `json:"episodeEnd"`
		MediaType    string `json:"mediaType"`
		TMDBID       string `json:"tmdbId"`
		Year         int    `json:"year"`
		Reference    string `json:"reference"`
	}{payload.SourceID, payload.CandidateID, payload.Season, payload.EpisodeStart, payload.EpisodeEnd, payload.MediaType, payload.TMDBID, payload.Year, payload.Reference})
	if err != nil {
		return Job{}, false, err
	}
	requestHash := sha256.Sum256(canonical)
	payload.Revision = 0
	payload.ExpiresAt = s.now().UTC().Add(jobTokenLifetime).Unix()
	storedToken, err := s.codec.Encode(payload)
	if err != nil {
		return Job{}, false, err
	}
	jobID, err := randomID()
	if err != nil {
		return Job{}, false, err
	}
	now := s.now().UTC().Unix()
	storedJob, created, err := s.store.CreateTransferJob(ctx, store.TransferJob{
		ID: jobID, UserID: userID, IdempotencyKey: idempotencyKey,
		RequestHash: requestHash[:], SelectionToken: storedToken,
		SourceID: payload.SourceID, CandidateID: payload.CandidateID,
		Title: payload.Title, Year: payload.Year, Season: payload.Season,
		EpisodeStart: payload.EpisodeStart, EpisodeEnd: payload.EpisodeEnd,
		MediaType: payload.MediaType, TMDBID: payload.TMDBID, State: "queued",
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return Job{}, false, err
	}
	if created {
		s.notify()
	}
	return publicJob(storedJob), created, nil
}

func (s *Service) Get(ctx context.Context, userID int64, jobID string) (JobDetail, error) {
	job, err := s.store.TransferJob(ctx, userID, jobID)
	if err != nil {
		return JobDetail{}, err
	}
	events, err := s.store.TransferEvents(ctx, userID, jobID)
	if err != nil {
		return JobDetail{}, err
	}
	publicEvents := make([]Event, 0, len(events))
	for _, event := range events {
		publicEvents = append(publicEvents, Event{
			ID: event.ID, State: event.State, Message: event.Message,
			CreatedAt: time.Unix(event.CreatedAt, 0).UTC(),
		})
	}
	return JobDetail{Job: publicJob(job), Events: publicEvents}, nil
}

func (s *Service) List(ctx context.Context, userID int64, limit int, archived bool) ([]Job, error) {
	jobs, err := s.store.ListTransferJobs(ctx, userID, limit, archived)
	if err != nil {
		return nil, err
	}
	result := make([]Job, 0, len(jobs))
	for _, job := range jobs {
		result = append(result, publicJob(job))
	}
	return result, nil
}

func (s *Service) SetArchived(ctx context.Context, userID int64, jobID string, archived bool) (Job, error) {
	job, err := s.store.SetTransferArchived(ctx, userID, jobID, archived, s.now())
	if err != nil {
		return Job{}, err
	}
	return publicJob(job), nil
}

func (s *Service) Delete(ctx context.Context, userID int64, jobID string) error {
	return s.store.DeleteTransferJob(ctx, userID, jobID)
}

func (s *Service) Retry(ctx context.Context, userID int64, jobID string) (Job, error) {
	settings.ProviderSettingsLock.RLock()
	defer settings.ProviderSettingsLock.RUnlock()
	job, err := s.store.RetryTransferJob(ctx, userID, jobID, s.now())
	if err != nil {
		return Job{}, err
	}
	s.notify()
	return publicJob(job), nil
}

func (s *Service) ListNotifications(ctx context.Context, userID int64, limit int) ([]Notification, error) {
	items, err := s.store.ListTransferNotifications(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	result := make([]Notification, 0, len(items))
	for _, item := range items {
		result = append(result, publicNotification(item))
	}
	return result, nil
}

func (s *Service) RetryNotification(
	ctx context.Context, userID int64, jobID, eventType, confirmation string,
) (Notification, error) {
	if confirmation != jobID+":"+eventType {
		return Notification{}, store.ErrNotificationNotRetryable
	}
	item, err := s.store.RetryTransferNotificationByUser(ctx, userID, jobID, eventType, s.now())
	if err != nil {
		return Notification{}, err
	}
	s.notify()
	return publicNotification(item), nil
}

func publicNotification(item store.TransferNotification) Notification {
	return Notification{
		ID: item.JobID + ":" + item.EventType, JobID: item.JobID, EventType: item.EventType,
		JobState: item.JobState, Title: item.Title, State: item.State, Attempts: item.Attempts,
		CreatedAt: time.Unix(item.CreatedAt, 0).UTC(), UpdatedAt: time.Unix(item.UpdatedAt, 0).UTC(),
	}
}

func (s *Service) notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func publicJob(job store.TransferJob) Job {
	return Job{
		ID: job.ID, Title: job.Title, Year: job.Year, Season: job.Season,
		EpisodeStart: job.EpisodeStart, EpisodeEnd: job.EpisodeEnd,
		MediaType: job.MediaType, TMDBID: job.TMDBID, Source: job.SourceID,
		State: job.State, ErrorCode: job.ErrorCode, ErrorMessage: job.ErrorMessage,
		Retryable: job.Retryable, Archived: job.ArchivedAt > 0, CreatedAt: time.Unix(job.CreatedAt, 0).UTC(),
		UpdatedAt: time.Unix(job.UpdatedAt, 0).UTC(),
	}
}

func randomID() (string, error) {
	raw := make([]byte, 18)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
