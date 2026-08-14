package workflow

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"media-hub/backend/internal/config"
	"media-hub/backend/internal/emby"
	"media-hub/backend/internal/qms"
	"media-hub/backend/internal/search"
	"media-hub/backend/internal/selection"
	"media-hub/backend/internal/store"
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
		qms.NewClient("http://qms.local", "qms-key", time.Second),
		emby.NewClient("http://emby.local", "emby-key", time.Second),
		nil,
		config.Workflow{QMediaSyncAccountID: 1, Movie: config.WorkflowTarget{DestinationID: "100", QMediaSyncTargetPath: "/strm/movies", EmbyLibraryID: "library-movies"}},
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

func TestUnknownQMediaSyncSubmissionIsNotAutomaticallyRepeated(t *testing.T) {
	ctx := context.Background()
	calls := 0
	qmsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/sync/manual" {
			calls++
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer qmsServer.Close()

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
		dataStore,
		searchService,
		codec,
		qms.NewClient(qmsServer.URL, "qms-key", time.Second),
		emby.NewClient("http://emby.local", "emby-key", time.Second),
		nil,
		config.Workflow{
			QMediaSyncAccountID: 3,
			Movie:               config.WorkflowTarget{DestinationID: "100", QMediaSyncTargetPath: "/strm/movies", EmbyLibraryID: "library-movies"},
		},
	)
	token := service.SelectionToken(search.Candidate{
		ID: "framehdr:item-1", Title: "Movie", MediaType: "movie", TMDBID: "123", SourceID: "framehdr",
		SourceRef: "private-reference", TransferState: "available", Revision: searchService.CurrentRevision(),
	})
	publicJob, _, err := service.Enqueue(ctx, admin.ID, token, "request_worker")
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
	if job.State != "needs_attention" || calls != 1 {
		t.Fatalf("state=%q submission calls=%d", job.State, calls)
	}
	if err := service.processJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("uncertain submission was repeated %d times", calls)
	}
}

func TestWorkflowCompletesOnlyAfterEmbyPlaybackIsReady(t *testing.T) {
	ctx := context.Background()
	qmsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/sync/manual":
			_, _ = w.Write([]byte(`{"code":200,"message":"ok"}`))
		case "/api/sync/records":
			_, _ = w.Write([]byte(`{"code":200,"data":{"total":1,"records":[{"id":1,"base_cid":"file-1","status":2,"created_at":2000000000,"finish_at":2000000001}]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer qmsServer.Close()
	embyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != "emby-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/Items/library-movies/Refresh":
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
		qms.NewClient(qmsServer.URL, "qms-key", time.Second),
		emby.NewClient(embyServer.URL, "emby-key", time.Second),
		nil,
		config.Workflow{
			QMediaSyncAccountID: 3,
			Movie:               config.WorkflowTarget{DestinationID: "100", QMediaSyncTargetPath: "/strm/movies", EmbyLibraryID: "library-movies"},
		},
	)
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
