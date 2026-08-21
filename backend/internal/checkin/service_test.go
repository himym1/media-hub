package checkin

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"media-hub/backend/internal/search"
	"media-hub/backend/internal/store"
)

type checkInStub struct {
	id      string
	label   string
	result  search.CheckInResult
	err     error
	calls   int
}

func (s *checkInStub) ID() string    { return s.id }
func (s *checkInStub) Label() string { return s.label }
func (s *checkInStub) Search(context.Context, string) ([]search.Candidate, error) {
	return nil, nil
}
func (s *checkInStub) CheckIn(context.Context) (search.CheckInResult, error) {
	s.calls++
	return s.result, s.err
}

type sourceFinderStub struct{ sources []search.CheckInSource }

func (s sourceFinderStub) CheckInSources() []search.CheckInSource { return s.sources }

type notifierStub struct {
	messages []string
}

func (n *notifierStub) Configured() bool { return true }
func (n *notifierStub) Send(_ context.Context, message string) (bool, error) {
	n.messages = append(n.messages, message)
	return false, nil
}

func TestSuccessfulCheckInNotifiesOnce(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	source := &checkInStub{id: "framehdr", label: "帧影", result: search.CheckInResult{State: "completed", Message: "签到成功"}}
	notifier := &notifierStub{}
	service := NewService(dataStore, sourceFinderStub{sources: []search.CheckInSource{source}}, notifier)
	service.now = func() time.Time { return time.Date(2026, 8, 21, 11, 0, 0, 0, checkInLocation) }
	if err := service.work(ctx); err != nil {
		t.Fatal(err)
	}
	if err := service.work(ctx); err != nil {
		t.Fatal(err)
	}
	if source.calls != 1 || len(notifier.messages) != 1 {
		t.Fatalf("calls=%d messages=%#v", source.calls, notifier.messages)
	}
	if notifier.messages[0] != "Media Hub\n帧影今日签到成功" {
		t.Fatalf("message=%q", notifier.messages[0])
	}
}

func TestAlreadyCheckedInNotifiesOnce(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	source := &checkInStub{id: "juying", label: "聚影", result: search.CheckInResult{State: "completed", Message: "今日已签到"}}
	notifier := &notifierStub{}
	service := NewService(dataStore, sourceFinderStub{sources: []search.CheckInSource{source}}, notifier)
	service.now = func() time.Time { return time.Date(2026, 8, 21, 11, 0, 0, 0, checkInLocation) }
	if err := service.work(ctx); err != nil {
		t.Fatal(err)
	}
	if len(notifier.messages) != 1 || notifier.messages[0] != "Media Hub\n聚影今日已签到" {
		t.Fatalf("messages=%#v", notifier.messages)
	}
}

func TestFailedCheckInThenSuccessNotifiesBoth(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	source := &checkInStub{
		id: "framehdr", label: "帧影",
		err: search.Failure{Code: "source_unavailable", Message: "帧影暂时不可用", Retryable: true},
	}
	notifier := &notifierStub{}
	now := time.Date(2026, 8, 21, 11, 0, 0, 0, checkInLocation)
	service := NewService(dataStore, sourceFinderStub{sources: []search.CheckInSource{source}}, notifier)
	service.now = func() time.Time { return now }
	if err := service.work(ctx); err != nil {
		t.Fatal(err)
	}
	source.err = nil
	source.result = search.CheckInResult{State: "completed", Message: "签到成功"}
	now = now.Add(2 * time.Minute)
	if err := service.work(ctx); err != nil {
		t.Fatal(err)
	}
	if len(notifier.messages) != 2 {
		t.Fatalf("messages=%#v", notifier.messages)
	}
	if notifier.messages[0] != "Media Hub\n帧影今日签到失败\n帧影暂时不可用" {
		t.Fatalf("failure=%q", notifier.messages[0])
	}
	if notifier.messages[1] != "Media Hub\n帧影今日签到成功" {
		t.Fatalf("success=%q", notifier.messages[1])
	}
}

func TestWorkerCompletesOncePerShanghaiDay(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	source := &checkInStub{id: "framehdr", label: "帧影", result: search.CheckInResult{State: "completed", Message: "签到成功"}}
	service := NewService(dataStore, sourceFinderStub{sources: []search.CheckInSource{source}}, nil)
	service.now = func() time.Time { return time.Date(2026, 8, 21, 11, 0, 0, 0, checkInLocation) }
	if err := service.work(ctx); err != nil {
		t.Fatal(err)
	}
	if err := service.work(ctx); err != nil {
		t.Fatal(err)
	}
	if source.calls != 1 {
		t.Fatalf("calls=%d", source.calls)
	}
	status, err := service.Status(ctx)
	if err != nil || len(status.Items) != 1 || status.Items[0].State != "completed" || status.Items[0].Message != "签到成功" {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	health := service.Check(ctx)
	if health.Status != "healthy" {
		t.Fatalf("health=%#v", health)
	}
}

func TestFailedCheckInNotifiesOnce(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	source := &checkInStub{
		id: "juying", label: "聚影",
		err: search.Failure{Code: "source_unauthorized", Message: "聚影用户名或密码无效", Retryable: false},
	}
	notifier := &notifierStub{}
	service := NewService(dataStore, sourceFinderStub{sources: []search.CheckInSource{source}}, notifier)
	service.now = func() time.Time { return time.Date(2026, 8, 21, 11, 0, 0, 0, checkInLocation) }
	if err := service.work(ctx); err != nil {
		t.Fatal(err)
	}
	if err := service.work(ctx); err != nil {
		t.Fatal(err)
	}
	if source.calls != 1 || len(notifier.messages) != 1 {
		t.Fatalf("calls=%d messages=%d", source.calls, len(notifier.messages))
	}
	if notifier.messages[0] != "Media Hub\n聚影今日签到失败\n聚影用户名或密码无效" {
		t.Fatalf("message=%q", notifier.messages[0])
	}
	health := service.Check(ctx)
	if health.Status != "degraded" || health.Detail != "聚影：聚影用户名或密码无效" {
		t.Fatalf("health=%#v", health)
	}
}

func TestWorkerSkipsBeforeScheduledTime(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	source := &checkInStub{id: "framehdr", label: "帧影", result: search.CheckInResult{State: "completed", Message: "签到成功"}}
	service := NewService(dataStore, sourceFinderStub{sources: []search.CheckInSource{source}}, nil)
	service.Configure(Schedule{Enabled: true, Hour: 8, Minute: 30, Sources: []string{"framehdr"}})
	service.now = func() time.Time { return time.Date(2026, 8, 21, 8, 29, 0, 0, checkInLocation) }
	if err := service.work(ctx); err != nil {
		t.Fatal(err)
	}
	if source.calls != 0 {
		t.Fatalf("called before schedule: %d", source.calls)
	}
	service.now = func() time.Time { return time.Date(2026, 8, 21, 8, 30, 0, 0, checkInLocation) }
	if err := service.work(ctx); err != nil {
		t.Fatal(err)
	}
	if source.calls != 1 {
		t.Fatalf("calls=%d", source.calls)
	}
}

func TestWorkerSkipsDisabledSchedule(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	source := &checkInStub{id: "framehdr", label: "帧影", result: search.CheckInResult{State: "completed", Message: "签到成功"}}
	service := NewService(dataStore, sourceFinderStub{sources: []search.CheckInSource{source}}, nil)
	service.Configure(Schedule{Enabled: false, Hour: 0, Minute: 5, Sources: []string{"framehdr"}})
	service.now = func() time.Time { return time.Date(2026, 8, 21, 11, 0, 0, 0, checkInLocation) }
	if err := service.work(ctx); err != nil {
		t.Fatal(err)
	}
	if source.calls != 0 {
		t.Fatalf("disabled auto check-in still ran: %d", source.calls)
	}
	item, err := service.Retry(ctx, "framehdr")
	if err != nil || item.State != "completed" || source.calls != 1 {
		t.Fatalf("manual retry item=%#v calls=%d err=%v", item, source.calls, err)
	}
}

func TestWorkerSkipsUnselectedSource(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	frame := &checkInStub{id: "framehdr", label: "帧影", result: search.CheckInResult{State: "completed", Message: "签到成功"}}
	juying := &checkInStub{id: "juying", label: "聚影", result: search.CheckInResult{State: "completed", Message: "签到成功"}}
	service := NewService(dataStore, sourceFinderStub{sources: []search.CheckInSource{frame, juying}}, nil)
	service.Configure(Schedule{Enabled: true, Hour: 0, Minute: 5, Sources: []string{"juying"}})
	service.now = func() time.Time { return time.Date(2026, 8, 21, 11, 0, 0, 0, checkInLocation) }
	if err := service.work(ctx); err != nil {
		t.Fatal(err)
	}
	if frame.calls != 0 || juying.calls != 1 {
		t.Fatalf("frame=%d juying=%d", frame.calls, juying.calls)
	}
}
