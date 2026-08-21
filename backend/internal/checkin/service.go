package checkin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"media-hub/backend/internal/integration"
	"media-hub/backend/internal/search"
	"media-hub/backend/internal/store"
)

const (
	workerInterval = 15 * time.Second
	maxAttempts    = 5
)

var (
	ErrUnavailable     = errors.New("source check-in is unavailable")
	ErrInvalidSource   = errors.New("source check-in id is invalid")
	checkInLocation    = loadCheckInLocation()
	sourceIDAllowlist  = func(id string) bool {
		if len(id) == 0 || len(id) > 50 {
			return false
		}
		for index, item := range id {
			if item >= 'a' && item <= 'z' || item >= '0' && item <= '9' {
				continue
			}
			if index > 0 && (item == '_' || item == '-') {
				continue
			}
			return false
		}
		return true
	}
)

type SourceFinder interface {
	CheckInSources() []search.CheckInSource
}

type Notifier interface {
	Configured() bool
	Send(context.Context, string) (bool, error)
}

type Item struct {
	SourceID      string     `json:"sourceId"`
	Label         string     `json:"label"`
	State         string     `json:"state"`
	Message       string     `json:"message,omitempty"`
	ErrorCode     string     `json:"errorCode,omitempty"`
	Retryable     bool       `json:"retryable"`
	LastSuccessAt *time.Time `json:"lastSuccessAt,omitempty"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type Status struct {
	Items []Item `json:"items"`
}

type Schedule struct {
	Enabled bool
	Hour    int
	Minute  int
	Sources []string
}

type Service struct {
	store    *store.Store
	search   SourceFinder
	notify   Notifier
	now      func() time.Time
	wake     chan struct{}
	done     chan struct{}
	mutex    sync.RWMutex
	schedule Schedule
}

func NewService(dataStore *store.Store, sources SourceFinder, notifier Notifier) *Service {
	return &Service{
		store: dataStore, search: sources, notify: notifier,
		now: time.Now, wake: make(chan struct{}, 1), done: make(chan struct{}),
		schedule: Schedule{Enabled: true, Hour: 0, Minute: 5, Sources: []string{"framehdr", "juying"}},
	}
}

func (s *Service) Configure(schedule Schedule) {
	if s == nil {
		return
	}
	s.mutex.Lock()
	s.schedule = Schedule{
		Enabled: schedule.Enabled, Hour: schedule.Hour, Minute: schedule.Minute,
		Sources: append([]string(nil), schedule.Sources...),
	}
	s.mutex.Unlock()
	s.requestWake()
}

func (s *Service) Start(ctx context.Context) error {
	if s == nil || s.store == nil {
		return ErrUnavailable
	}
	if err := s.store.RecoverInterruptedCheckIns(ctx, s.now()); err != nil {
		return err
	}
	go func() {
		defer close(s.done)
		s.run(ctx)
	}()
	return nil
}

func (s *Service) Wait(ctx context.Context) error {
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	if s == nil || s.store == nil {
		return Status{}, ErrUnavailable
	}
	rows, err := s.store.ListSourceCheckIns(ctx)
	if err != nil {
		return Status{}, err
	}
	byID := make(map[string]store.SourceCheckIn, len(rows))
	for _, row := range rows {
		byID[row.SourceID] = row
	}
	schedule := s.currentSchedule()
	items := make([]Item, 0)
	for _, source := range s.checkInSources() {
		item := Item{
			SourceID: source.ID(), Label: source.Label(), State: "idle",
			Message: idleCheckInMessage(schedule, source.ID()), UpdatedAt: s.now().UTC(),
		}
		if row, ok := byID[source.ID()]; ok {
			item = publicCheckIn(row)
		}
		items = append(items, item)
	}
	return Status{Items: items}, nil
}

func (s *Service) Retry(ctx context.Context, sourceID string) (Item, error) {
	if s == nil || s.store == nil {
		return Item{}, ErrUnavailable
	}
	if !sourceIDAllowlist(sourceID) {
		return Item{}, ErrInvalidSource
	}
	found := false
	for _, source := range s.checkInSources() {
		if source.ID() == sourceID {
			found = true
			break
		}
	}
	if !found {
		return Item{}, store.ErrCheckInNotFound
	}
	if _, err := s.store.RetrySourceCheckIn(ctx, sourceID, s.now()); err != nil && !errors.Is(err, store.ErrCheckInNotFound) {
		return Item{}, err
	}
	var selected search.CheckInSource
	for _, source := range s.checkInSources() {
		if source.ID() == sourceID {
			selected = source
			break
		}
	}
	if err := s.process(ctx, selected, true); err != nil {
		return Item{}, err
	}
	row, err := s.store.SourceCheckIn(ctx, sourceID)
	if err != nil {
		return Item{}, err
	}
	return publicCheckIn(row), nil
}

func (s *Service) Check(ctx context.Context) integration.Health {
	health := integration.Health{ID: "source-checkin", Label: "资源签到"}
	if s == nil || s.search == nil {
		health.Status = integration.StatusUnconfigured
		health.Detail = "签到服务未配置"
		return health
	}
	schedule := s.currentSchedule()
	sources := s.checkInSources()
	if len(sources) == 0 {
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未配置可签到的资源源"
		return health
	}
	if !schedule.Enabled {
		health.Status = integration.StatusHealthy
		health.Detail = "自动签到已关闭，可手动签到"
		return health
	}
	status, err := s.Status(ctx)
	if err != nil {
		health.Status = integration.StatusUnavailable
		health.Detail = "无法读取签到状态"
		return health
	}
	failed := 0
	done := 0
	var detail string
	for _, item := range status.Items {
		switch item.State {
		case "failed", "needs_attention":
			failed++
			if detail == "" {
				detail = item.Label + "：" + item.Message
			}
		case "completed", "skipped":
			done++
		}
	}
	switch {
	case failed > 0:
		health.Status = integration.StatusDegraded
		health.Detail = detail
	case done == len(status.Items):
		health.Status = integration.StatusHealthy
		health.Detail = "今日资源源签到已完成"
	default:
		health.Status = integration.StatusHealthy
		health.Detail = "已配置自动签到，等待执行"
	}
	return health
}

func (s *Service) run(ctx context.Context) {
	ticker := time.NewTicker(workerInterval)
	defer ticker.Stop()
	for {
		_ = s.work(ctx)
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-ticker.C:
		}
	}
}

func (s *Service) work(ctx context.Context) error {
	schedule := s.currentSchedule()
	if !schedule.Enabled || !afterScheduledTime(s.now(), schedule) {
		return nil
	}
	for _, source := range s.checkInSources() {
		if !scheduleAllows(schedule, source.ID()) {
			continue
		}
		if err := s.process(ctx, source, false); err != nil {
			return err
		}
		s.retryNotify(ctx, source.ID())
	}
	return nil
}

func (s *Service) process(ctx context.Context, source search.CheckInSource, force bool) error {
	now := s.now()
	day := checkInDay(now)
	row, claimed, err := s.store.ClaimSourceCheckIn(ctx, source.ID(), source.Label(), day, now, force)
	if err != nil || !claimed {
		return err
	}
	result, checkErr := source.CheckIn(ctx)
	row.Label = source.Label()
	row.UpdatedAt = s.now().UTC().Unix()
	if checkErr != nil {
		s.applyFailure(&row, checkErr, day, now)
	} else {
		s.applyResult(&row, result, day, now)
	}
	if err := s.store.FinishSourceCheckIn(ctx, row); err != nil {
		return err
	}
	s.notifyResult(ctx, row, day)
	return nil
}

func (s *Service) applyResult(row *store.SourceCheckIn, result search.CheckInResult, day string, now time.Time) {
	state := strings.TrimSpace(result.State)
	if state == "" {
		state = "completed"
	}
	row.State = state
	row.Message = strings.TrimSpace(result.Message)
	row.ErrorCode = ""
	row.Retryable = false
	row.Attempts = 0
	row.NextAttemptAt = 0
	row.LastDay = day
	if state == "completed" {
		row.LastSuccessAt = now.UTC().Unix()
	}
}

func (s *Service) applyFailure(row *store.SourceCheckIn, err error, day string, now time.Time) {
	failure := search.Failure{Code: "checkin_failed", Message: "资源源签到失败", Retryable: true}
	var typed search.Failure
	if errors.As(err, &typed) {
		failure = typed
	}
	row.ErrorCode = failure.Code
	row.Message = failure.Message
	row.Retryable = failure.Retryable
	if failure.Code == "checkin_unknown" || failure.Code == "source_access_unknown" || failure.Code == "source_submission_unknown" {
		row.State = "needs_attention"
		row.NextAttemptAt = 0
		row.LastDay = day
		return
	}
	if !failure.Retryable || row.Attempts >= maxAttempts {
		row.State = "failed"
		row.NextAttemptAt = 0
		row.LastDay = day
		return
	}
	row.State = "failed"
	row.NextAttemptAt = now.UTC().Add(checkInBackoff(row.Attempts)).Unix()
}

func (s *Service) retryNotify(ctx context.Context, sourceID string) {
	row, err := s.store.SourceCheckIn(ctx, sourceID)
	if err != nil {
		return
	}
	day := checkInDay(s.now())
	if row.LastDay != day {
		return
	}
	s.notifyResult(ctx, row, day)
}

func (s *Service) notifyResult(ctx context.Context, row store.SourceCheckIn, day string) {
	if s.notify == nil || !s.notify.Configured() {
		return
	}
	message := checkInNotificationMessage(row)
	if message == "" {
		return
	}
	if row.NotifiedDay == day && row.NotifiedState == row.State {
		return
	}
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
	defer cancel()
	if unknown, err := s.notify.Send(sendCtx, message); err != nil || unknown {
		return
	}
	_ = s.store.MarkSourceCheckInNotified(ctx, row.SourceID, day, row.State, s.now())
}

func checkInNotificationMessage(row store.SourceCheckIn) string {
	switch row.State {
	case "completed":
		line := row.Label + "今日签到成功"
		if row.Message == "今日已签到" {
			line = row.Label + "今日已签到"
		}
		return "Media Hub\n" + line
	case "failed", "needs_attention":
		message := "Media Hub\n" + row.Label + "今日签到失败"
		if row.Message != "" {
			message += "\n" + row.Message
		}
		return message
	default:
		return ""
	}
}

func (s *Service) checkInSources() []search.CheckInSource {
	if s.search == nil {
		return nil
	}
	return s.search.CheckInSources()
}

func (s *Service) currentSchedule() Schedule {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return Schedule{
		Enabled: s.schedule.Enabled, Hour: s.schedule.Hour, Minute: s.schedule.Minute,
		Sources: append([]string(nil), s.schedule.Sources...),
	}
}

func afterScheduledTime(now time.Time, schedule Schedule) bool {
	local := now.In(checkInLocation)
	scheduled := time.Date(local.Year(), local.Month(), local.Day(), schedule.Hour, schedule.Minute, 0, 0, checkInLocation)
	return !local.Before(scheduled)
}

func idleCheckInMessage(schedule Schedule, sourceID string) string {
	if !schedule.Enabled {
		return "自动签到已关闭，可手动签到"
	}
	if !scheduleAllows(schedule, sourceID) {
		return "未加入自动签到，可手动签到"
	}
	return fmt.Sprintf("等待今日 %02d:%02d 自动签到", schedule.Hour, schedule.Minute)
}

func scheduleAllows(schedule Schedule, sourceID string) bool {
	if len(schedule.Sources) == 0 {
		return false
	}
	for _, candidate := range schedule.Sources {
		if candidate == sourceID {
			return true
		}
	}
	return false
}

func (s *Service) requestWake() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func publicCheckIn(row store.SourceCheckIn) Item {
	item := Item{
		SourceID: row.SourceID, Label: row.Label, State: row.State, Message: row.Message,
		ErrorCode: row.ErrorCode, Retryable: row.Retryable, UpdatedAt: time.Unix(row.UpdatedAt, 0).UTC(),
	}
	if row.LastSuccessAt > 0 {
		value := time.Unix(row.LastSuccessAt, 0).UTC()
		item.LastSuccessAt = &value
	}
	return item
}

func checkInDay(now time.Time) string {
	return now.In(checkInLocation).Format("2006-01-02")
}

func checkInBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > maxAttempts {
		attempt = maxAttempts
	}
	return time.Duration(1<<(attempt-1)) * time.Minute
}

func loadCheckInLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return location
}

