package workflow

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"media-hub/backend/internal/config"
	"media-hub/backend/internal/emby"
	"media-hub/backend/internal/search"
	"media-hub/backend/internal/selection"
	"media-hub/backend/internal/store"
	"media-hub/backend/internal/strm"
)

type accessUnknownSourceStub struct{ calls *int }

func (s accessUnknownSourceStub) ID() string    { return "juying" }
func (s accessUnknownSourceStub) Label() string { return "聚影" }
func (s accessUnknownSourceStub) Search(context.Context, string) ([]search.Candidate, error) {
	return nil, nil
}
func (s accessUnknownSourceStub) StartTransfer(context.Context, search.TransferRequest) (search.TransferResult, error) {
	*s.calls++
	return search.TransferResult{}, search.Failure{Code: "source_access_unknown", Message: "聚影资源访问结果未知，需要确认后重试", Retryable: true}
}
func (s accessUnknownSourceStub) TransferStatus(context.Context, int64, string) (search.TransferResult, error) {
	return search.TransferResult{}, nil
}

func TestUnknownSourceAccessIsNotAutomaticallyRepeated(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	if _, err := dataStore.EnsureAdmin(ctx, "password-hash"); err != nil {
		t.Fatal(err)
	}
	admin, _, err := dataStore.Admin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	codec, err := selection.NewCodec(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	searchService := search.NewService(accessUnknownSourceStub{calls: &calls})
	service := NewService(
		dataStore, searchService, codec,
		emby.NewClient("http://emby.local", "emby-key", time.Second),
		nil,
		config.Workflow{
			StrmBaseURL:   "https://media.example",
			StrmRootMount: "/media",
			Movie:         config.WorkflowTarget{DestinationID: "100", QMediaSyncTargetPath: "/strm/movies", EmbyLibraryID: "library-movies"},
		},
		nil,
		nil,
		nil,
	)
	token := service.SelectionToken(search.Candidate{
		ID: "juying:item-1", Title: "Movie", MediaType: "movie", TMDBID: "123", SourceID: "juying",
		SourceRef: `{"kind":"web","movieId":"1","resourceId":"2"}`, TransferState: "available", Revision: searchService.CurrentRevision(),
	})
	publicJob, _, err := service.Enqueue(ctx, admin.ID, token, "request_access_unknown")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		job, err := dataStore.TransferJob(ctx, admin.ID, publicJob.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := service.processJob(ctx, job); err != nil {
			t.Fatal(err)
		}
	}
	job, err := dataStore.TransferJob(ctx, admin.ID, publicJob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if job.State != "needs_attention" || job.ErrorCode != "source_access_unknown" || calls != 1 {
		t.Fatalf("state=%q code=%q calls=%d", job.State, job.ErrorCode, calls)
	}
	if err := service.processJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("uncertain access was repeated %d times", calls)
	}
	retried, err := service.Retry(ctx, admin.ID, publicJob.ID)
	if err != nil || retried.State != "transferring" {
		t.Fatalf("explicit retry state=%q err=%v", retried.State, err)
	}
	job, err = dataStore.TransferJob(ctx, admin.ID, publicJob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.processJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	job, err = dataStore.TransferJob(ctx, admin.ID, publicJob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if job.State != "needs_attention" || calls != 2 {
		t.Fatalf("explicit retry state=%q calls=%d", job.State, calls)
	}
}

func TestWorkflowCompletesOnlyAfterEmbyPlaybackIsReady(t *testing.T) {
	ctx := context.Background()
	embyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != "emby-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/Items/library-movies/Refresh":
			w.WriteHeader(http.StatusNoContent)
		case "/Items/RemoteSearch/Apply/emby-item":
			if request.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case "/Items":
			_, _ = w.Write([]byte(`{"Items":[{"Id":"emby-item","Name":"Movie","Type":"Movie","ProductionYear":2026}],"TotalRecordCount":1}`))
		case "/Items/emby-item/PlaybackInfo":
			_, _ = w.Write([]byte(`{"MediaSources":[{"Id":"media-source"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer embyServer.Close()

	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	if _, err := dataStore.EnsureAdmin(ctx, "password-hash"); err != nil {
		t.Fatal(err)
	}
	admin, _, err := dataStore.Admin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	codec, err := selection.NewCodec(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatal(err)
	}
	searchService := search.NewService(transferSourceStub{})
	service := NewService(
		dataStore, searchService, codec,
		emby.NewClient(embyServer.URL, "emby-key", time.Second),
		nil,
		config.Workflow{
			StrmBaseURL:   "https://media.example",
			StrmRootMount: "/media",
			Movie:         config.WorkflowTarget{DestinationID: "100", QMediaSyncTargetPath: "/strm/movies", EmbyLibraryID: "library-movies"},
		},
		func(context.Context, string) (string, error) { return "Media/Movies", nil },
		nil,
		nil,
	)
	service.UseSTRMSyncer(&strmStub{})
	token := service.SelectionToken(search.Candidate{
		ID: "framehdr:item-1", Title: "Movie", Year: 2026, MediaType: "movie", TMDBID: "123", SourceID: "framehdr",
		SourceRef: "private-reference", TransferState: "available", Revision: searchService.CurrentRevision(),
	})
	publicJob, _, err := service.Enqueue(ctx, admin.ID, token, "request_complete")
	if err != nil {
		t.Fatal(err)
	}

	for range 8 {
		job, err := dataStore.TransferJob(ctx, admin.ID, publicJob.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := service.processJob(ctx, job); err != nil {
			t.Fatal(err)
		}
	}
	job, err := dataStore.TransferJob(ctx, admin.ID, publicJob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if job.State != "completed" || job.EmbyItemID != "emby-item" {
		t.Fatalf("state=%q emby item=%q", job.State, job.EmbyItemID)
	}
}

type strmStub struct {
	calls   int
	request strm.Request
	err     error
}

func (s *strmStub) Sync(_ context.Context, request strm.Request) (strm.Result, error) {
	s.calls++
	s.request = request
	if s.err != nil {
		return strm.Result{}, s.err
	}
	return strm.Result{Created: 1}, nil
}

func TestBuiltinSyncWritesSTRMAndRefreshesEmby(t *testing.T) {
	ctx := context.Background()
	embyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != "emby-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/Items/library-movies/Refresh":
			w.WriteHeader(http.StatusNoContent)
		case "/Items/RemoteSearch/Apply/emby-item":
			w.WriteHeader(http.StatusNoContent)
		case "/Items":
			_, _ = w.Write([]byte(`{"Items":[{"Id":"emby-item","Name":"Movie","Type":"Movie","ProductionYear":2026}],"TotalRecordCount":1}`))
		case "/Items/emby-item/PlaybackInfo":
			_, _ = w.Write([]byte(`{"MediaSources":[{"Id":"media-source"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer embyServer.Close()

	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	if _, err := dataStore.EnsureAdmin(ctx, "password-hash"); err != nil {
		t.Fatal(err)
	}
	admin, _, err := dataStore.Admin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	codec, err := selection.NewCodec(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatal(err)
	}
	searchService := search.NewService(transferSourceStub{})
	syncer := &strmStub{}
	service := NewService(
		dataStore, searchService, codec,
		emby.NewClient(embyServer.URL, "emby-key", time.Second),
		nil,
		config.Workflow{
			SyncMode:      config.SyncModeBuiltin,
			StrmBaseURL:   "https://media.example",
			StrmRootMount: "/media",
			Movie:         config.WorkflowTarget{DestinationID: "100", QMediaSyncTargetPath: "/media/电影", EmbyLibraryID: "library-movies"},
		},
		func(context.Context, string) (string, error) { return "电影/Movie (2018)", nil },
		nil,
		nil,
	)
	service.UseSTRMSyncer(syncer)
	token := service.SelectionToken(search.Candidate{
		ID: "framehdr:item-1", Title: "Movie", Year: 2026, MediaType: "movie", TMDBID: "123", SourceID: "framehdr",
		SourceRef: "private-reference", TransferState: "available", Revision: searchService.CurrentRevision(),
	})
	publicJob, _, err := service.Enqueue(ctx, admin.ID, token, "request_builtin")
	if err != nil {
		t.Fatal(err)
	}
	for range 7 {
		job, err := dataStore.TransferJob(ctx, admin.ID, publicJob.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := service.processJob(ctx, job); err != nil {
			t.Fatal(err)
		}
	}
	job, err := dataStore.TransferJob(ctx, admin.ID, publicJob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if job.State != "completed" || syncer.calls != 1 {
		t.Fatalf("state=%q strm=%d", job.State, syncer.calls)
	}
	if syncer.request.FileID != "file-1" || syncer.request.TargetPath != "/media/电影" || syncer.request.StrmBaseURL != "https://media.example" || !syncer.request.Prune {
		t.Fatalf("request=%#v", syncer.request)
	}
	events, err := dataStore.TransferEvents(ctx, admin.ID, publicJob.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		if strings.Contains(event.Message, "STRM 同步完成") && strings.Contains(event.Message, "新建 1") {
			found = true
		}
	}
	if !found {
		t.Fatalf("events=%#v", events)
	}
}

func TestBuiltinSyncAuthExpiryIsRetryable(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	if _, err := dataStore.EnsureAdmin(ctx, "password-hash"); err != nil {
		t.Fatal(err)
	}
	admin, _, err := dataStore.Admin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	codec, err := selection.NewCodec(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatal(err)
	}
	searchService := search.NewService(transferSourceStub{})
	service := NewService(
		dataStore, searchService, codec,
		emby.NewClient("http://emby.local", "emby-key", time.Second),
		nil,
		config.Workflow{
			SyncMode:      config.SyncModeBuiltin,
			StrmBaseURL:   "https://media.example",
			StrmRootMount: "/media",
			Movie:         config.WorkflowTarget{DestinationID: "100", QMediaSyncTargetPath: "/media/电影", EmbyLibraryID: "library-movies"},
		},
		func(context.Context, string) (string, error) { return "电影/Movie (2018)", nil },
		nil,
		nil,
	)
	service.UseSTRMSyncer(&strmStub{err: strm.ErrAuthExpired})
	token := service.SelectionToken(search.Candidate{
		ID: "framehdr:item-1", Title: "Movie", Year: 2026, MediaType: "movie", TMDBID: "123", SourceID: "framehdr",
		SourceRef: "private-reference", TransferState: "available", Revision: searchService.CurrentRevision(),
	})
	publicJob, _, err := service.Enqueue(ctx, admin.ID, token, "request_strm_auth")
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		job, err := dataStore.TransferJob(ctx, admin.ID, publicJob.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := service.processJob(ctx, job); err != nil {
			t.Fatal(err)
		}
	}
	job, err := dataStore.TransferJob(ctx, admin.ID, publicJob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if job.State != "failed" || job.ErrorCode != "strm_auth_expired" || !job.Retryable || job.ResumeState != "transferred" {
		t.Fatalf("state=%q code=%q retryable=%v resume=%q", job.State, job.ErrorCode, job.Retryable, job.ResumeState)
	}
}

func TestNotificationMessageIncludesJobError(t *testing.T) {
	message := notificationMessage(store.TransferNotification{
		Title: "Movie", EventType: "failed",
		ErrorMessage: "115 授权已失效，请在概览页重新扫码后再重试任务",
	})
	if message != "Media Hub\n《Movie》处理失败\n115 授权已失效，请在概览页重新扫码后再重试任务" {
		t.Fatalf("message=%q", message)
	}
}
