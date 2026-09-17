package workflow

import (
	"context"
	"encoding/base64"
	"errors"
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
	message := (&Service{}).notificationMessage(context.Background(), store.TransferNotification{
		Title: "Movie", EventType: "failed",
		ErrorMessage: "115 授权已失效，请在概览页重新扫码后再重试任务",
	})
	if message != "Media Hub\n《Movie》处理失败\n115 授权已失效，请在概览页重新扫码后再重试任务" {
		t.Fatalf("message=%q", message)
	}
}

type libraryRootTransferStub struct{}

func (libraryRootTransferStub) ID() string    { return "framehdr" }
func (libraryRootTransferStub) Label() string { return "帧影" }
func (libraryRootTransferStub) Search(context.Context, string) ([]search.Candidate, error) {
	return nil, nil
}
func (libraryRootTransferStub) StartTransfer(context.Context, search.TransferRequest) (search.TransferResult, error) {
	return search.TransferResult{Status: "completed", FileID: "100", Path: "/cloud/电影", IsFile: false}, nil
}
func (libraryRootTransferStub) TransferStatus(context.Context, int64, string) (search.TransferResult, error) {
	return search.TransferResult{}, nil
}

func TestSubmitSyncDoesNotRenameLibraryRoot(t *testing.T) {
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
	searchService := search.NewService(libraryRootTransferStub{})
	syncer := &strmStub{}
	renames := 0
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
		func(context.Context, string) (string, error) { return "/cloud/电影", nil },
		func(_ context.Context, fileID, name string) error {
			renames++
			t.Fatalf("renamed library root %s to %s", fileID, name)
			return nil
		},
		nil,
	)
	service.UseSTRMSyncer(syncer)
	token := service.SelectionToken(search.Candidate{
		ID: "framehdr:item-1", Title: "寻找艾米丽", Year: 2026, MediaType: "movie", TMDBID: "1339588", SourceID: "framehdr",
		SourceRef: "private-reference", TransferState: "available", Revision: searchService.CurrentRevision(),
	})
	publicJob, _, err := service.Enqueue(ctx, admin.ID, token, "request_library_root")
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
	if renames != 0 {
		t.Fatalf("renames=%d", renames)
	}
	if syncer.calls != 1 || syncer.request.FileID != "100" {
		t.Fatalf("strm=%#v", syncer.request)
	}
}

func TestMovieEmbyIndexDoesNotRequirePlaybackInfo(t *testing.T) {
	ctx := context.Background()
	playbackHits := 0
	embyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != "emby-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/Items":
			_, _ = w.Write([]byte(`{"Items":[{"Id":"emby-item","Name":"Movie","Type":"Movie","ProductionYear":2026,"ProviderIds":{"Tmdb":"123"}}],"TotalRecordCount":1}`))
		case "/Items/emby-item/PlaybackInfo":
			playbackHits++
			w.WriteHeader(http.StatusInternalServerError)
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
	service := NewService(
		dataStore, search.NewService(transferSourceStub{}), codec,
		emby.NewClient(embyServer.URL, "emby-key", time.Second),
		nil,
		config.Workflow{
			Movie: config.WorkflowTarget{DestinationID: "100", QMediaSyncTargetPath: "/media/电影", EmbyLibraryID: "library-movies"},
		},
		nil, nil, nil,
	)
	job := store.TransferJob{
		ID: "job-index-playback", UserID: admin.ID, IdempotencyKey: "request_index_playback",
		RequestHash: []byte("hash"), SelectionToken: "encrypted", SourceID: "sidhub",
		CandidateID: "candidate", Title: "Movie", Year: 2026, MediaType: "movie", TMDBID: "123",
		State: "queued", CreatedAt: 1, UpdatedAt: 1,
	}
	if _, _, err := dataStore.CreateTransferJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	job.State = "indexing_emby"
	job.UpdatedAt = 2
	if updated, err := dataStore.UpdateTransferJob(ctx, job, "queued", "setup index"); err != nil || !updated {
		t.Fatalf("prepare index job: updated=%v err=%v", updated, err)
	}
	loaded, err := dataStore.TransferJob(ctx, admin.ID, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.processJob(ctx, loaded); err != nil {
		t.Fatal(err)
	}
	loaded, err = dataStore.TransferJob(ctx, admin.ID, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.State != "verifying_playback" || loaded.EmbyItemID != "emby-item" {
		t.Fatalf("state=%q emby=%q", loaded.State, loaded.EmbyItemID)
	}
	if playbackHits != 0 {
		t.Fatalf("playback info hits=%d", playbackHits)
	}
}

func TestAdultIndexCompletesWithoutScrapeWhenUndetected(t *testing.T) {
	ctx := context.Background()
	refreshHits := 0
	embyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != "emby-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if strings.HasSuffix(request.URL.Path, "/Refresh") {
			refreshHits++
			w.WriteHeader(http.StatusNoContent)
			return
		}
		switch request.URL.Path {
		case "/Items", "/Users/user-1/Views", "/Library/MediaFolders":
			_, _ = w.Write([]byte(`{"Items":[],"TotalRecordCount":0}`))
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
	service := NewService(
		dataStore, search.NewService(transferSourceStub{}), codec,
		emby.NewClient(embyServer.URL, "emby-key", time.Second),
		nil,
		config.Workflow{
			Adult: config.WorkflowTarget{DestinationID: "300", QMediaSyncTargetPath: "/strm/adult", EmbyLibraryID: "adult"},
		},
		nil, nil, nil,
	)
	job := store.TransferJob{
		ID: "job-adult-skip-scrape", UserID: admin.ID, IdempotencyKey: "request_adult_skip",
		RequestHash: []byte("hash"), SelectionToken: "encrypted", SourceID: "share",
		CandidateID: "uploaded-1", Title: "127.0.0.1", MediaType: "adult",
		State: "queued", CreatedAt: 1, UpdatedAt: 1,
	}
	if _, _, err := dataStore.CreateTransferJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	job.State = "indexing_emby"
	job.UpdatedAt = 2
	if updated, err := dataStore.UpdateTransferJob(ctx, job, "queued", "setup index"); err != nil || !updated {
		t.Fatalf("prepare index job: updated=%v err=%v", updated, err)
	}
	loaded, err := dataStore.TransferJob(ctx, admin.ID, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.processJob(ctx, loaded); err != nil {
		t.Fatal(err)
	}
	loaded, err = dataStore.TransferJob(ctx, admin.ID, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.State != "completed" || loaded.EmbyItemID != "" || refreshHits != 0 {
		t.Fatalf("state=%q emby=%q refresh=%d", loaded.State, loaded.EmbyItemID, refreshHits)
	}
}

func TestAdultIndexScrapesOnceWhenCodeDetected(t *testing.T) {
	ctx := context.Background()
	refreshHits := 0
	embyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != "emby-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if request.URL.Path == "/Items/emby-adult/Refresh" {
			refreshHits++
			if request.URL.Query().Get("MetadataRefreshMode") != "FullRefresh" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		switch request.URL.Path {
		case "/Items":
			_, _ = w.Write([]byte(`{"Items":[{"Id":"emby-adult","Name":"SSIS-001","Type":"Video","Path":"/strm/adult/SSIS-001/clip.strm"}],"TotalRecordCount":1}`))
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
	service := NewService(
		dataStore, search.NewService(transferSourceStub{}), codec,
		emby.NewClient(embyServer.URL, "emby-key", time.Second),
		nil,
		config.Workflow{
			Adult: config.WorkflowTarget{DestinationID: "300", QMediaSyncTargetPath: "/strm/adult", EmbyLibraryID: "adult"},
		},
		nil, nil, nil,
	)
	job := store.TransferJob{
		ID: "job-adult-code", UserID: admin.ID, IdempotencyKey: "request_adult_code",
		RequestHash: []byte("hash"), SelectionToken: "encrypted", SourceID: "share",
		CandidateID: "uploaded-2", Title: "SSIS-001", MediaType: "adult",
		State: "queued", CreatedAt: 1, UpdatedAt: 1,
	}
	if _, _, err := dataStore.CreateTransferJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	job.State = "indexing_emby"
	job.UpdatedAt = 2
	if updated, err := dataStore.UpdateTransferJob(ctx, job, "queued", "setup index"); err != nil || !updated {
		t.Fatalf("prepare index job: updated=%v err=%v", updated, err)
	}
	loaded, err := dataStore.TransferJob(ctx, admin.ID, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.processJob(ctx, loaded); err != nil {
		t.Fatal(err)
	}
	loaded, err = dataStore.TransferJob(ctx, admin.ID, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.State != "completed" || loaded.EmbyItemID != "emby-adult" || refreshHits != 1 {
		t.Fatalf("state=%q emby=%q refresh=%d", loaded.State, loaded.EmbyItemID, refreshHits)
	}
}

type offlineFolderSourceStub struct{ starts *int }

func (s offlineFolderSourceStub) ID() string    { return "sidhub" }
func (s offlineFolderSourceStub) Label() string { return "Sidhub" }
func (s offlineFolderSourceStub) Search(context.Context, string) ([]search.Candidate, error) {
	return nil, nil
}
func (s offlineFolderSourceStub) StartTransfer(context.Context, search.TransferRequest) (search.TransferResult, error) {
	*s.starts++
	return search.TransferResult{OperationID: "op-1", Status: "pending", FileID: "folder-1", Path: "异魔禁区", IsFile: false}, nil
}
func (s offlineFolderSourceStub) TransferStatus(context.Context, int64, string) (search.TransferResult, error) {
	return search.TransferResult{}, search.Failure{Code: "invalid_source_response", Message: "should not poll sidhub status", Retryable: false}
}

func TestOfflineFolderWaitsForVideosBeforeTransferComplete(t *testing.T) {
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
	starts := 0
	searchService := search.NewService(offlineFolderSourceStub{starts: &starts})
	service := NewService(
		dataStore, searchService, codec,
		emby.NewClient("http://emby.local", "emby-key", time.Second),
		nil,
		config.Workflow{
			StrmBaseURL:   "https://media.example",
			StrmRootMount: "/media",
			Movie:         config.WorkflowTarget{DestinationID: "100", QMediaSyncTargetPath: "/strm/movies", EmbyLibraryID: "library-movies"},
		},
		nil, nil, nil,
	)
	ready := false
	service.UseFolderVideos(func(context.Context, string) (bool, error) { return ready, nil })
	token := service.SelectionToken(search.Candidate{
		ID: "sidhub:seed-1", Title: "异魔禁区", Year: 2001, MediaType: "movie", TMDBID: "1", SourceID: "sidhub",
		SourceRef: `{"title":"异魔禁区","linkPath":"/link_start/?seed_id=1"}`, TransferState: "available", Revision: searchService.CurrentRevision(),
	})
	publicJob, _, err := service.Enqueue(ctx, admin.ID, token, "request_offline_wait")
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
	if job.State != "transferring" || starts != 1 {
		t.Fatalf("waiting state=%q starts=%d", job.State, starts)
	}
	detail, err := service.Get(ctx, admin.ID, publicJob.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundWait := false
	for _, event := range detail.Events {
		if event.Message == "已提交 115 离线，等待视频到账" {
			foundWait = true
		}
		if event.Message == "资源转存完成" {
			t.Fatal("empty folder was marked transferred")
		}
	}
	if !foundWait {
		t.Fatal("missing offline wait event")
	}
	ready = true
	if err := service.processJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	job, err = dataStore.TransferJob(ctx, admin.ID, publicJob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if job.State != "transferred" || starts != 1 {
		t.Fatalf("ready state=%q starts=%d", job.State, starts)
	}
}

func TestOfflineFolderFailsWhenVideosNeverArrive(t *testing.T) {
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
	starts := 0
	searchService := search.NewService(offlineFolderSourceStub{starts: &starts})
	service := NewService(
		dataStore, searchService, codec,
		emby.NewClient("http://emby.local", "emby-key", time.Second),
		nil,
		config.Workflow{
			StrmBaseURL:   "https://media.example",
			StrmRootMount: "/media",
			Movie:         config.WorkflowTarget{DestinationID: "100", QMediaSyncTargetPath: "/strm/movies", EmbyLibraryID: "library-movies"},
		},
		nil, nil, nil,
	)
	service.UseFolderVideos(func(context.Context, string) (bool, error) { return false, nil })
	token := service.SelectionToken(search.Candidate{
		ID: "sidhub:seed-2", Title: "异魔禁区", Year: 2001, MediaType: "movie", TMDBID: "1", SourceID: "sidhub",
		SourceRef: `{"title":"异魔禁区","linkPath":"/link_start/?seed_id=2"}`, TransferState: "available", Revision: searchService.CurrentRevision(),
	})
	if token == "" {
		t.Fatal("empty selection token")
	}
	publicJob, _, err := service.Enqueue(ctx, admin.ID, token, "request_offline_timeout")
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
	if job.State != "transferring" || starts != 1 {
		t.Fatalf("waiting state=%q starts=%d", job.State, starts)
	}
	created := time.Unix(job.CreatedAt, 0).UTC()
	service.now = func() time.Time { return created.Add(offlineWaitTimeout + time.Second) }
	if err := service.processJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	job, err = dataStore.TransferJob(ctx, admin.ID, publicJob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if job.State != "failed" || job.ErrorCode != "offline_incomplete" || !job.Retryable {
		t.Fatalf("state=%q code=%q retryable=%v", job.State, job.ErrorCode, job.Retryable)
	}
	if job.ErrorMessage != "115 离线未完成，目录里还没有视频" {
		t.Fatalf("message=%q", job.ErrorMessage)
	}
	detail, err := service.Get(ctx, admin.ID, publicJob.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range detail.Events {
		if event.Message == "资源转存完成" {
			t.Fatal("empty folder was marked transferred")
		}
	}
}

type subtitleAttachStub struct {
	ids    []string
	status string
	err    error
}

func (s *subtitleAttachStub) AttachChinese(_ context.Context, itemID string) (string, error) {
	s.ids = append(s.ids, itemID)
	if s.err != nil {
		return "", s.err
	}
	if s.status == "" {
		return "attached", nil
	}
	return s.status, nil
}

func TestMovieIndexAttachesChineseSubtitles(t *testing.T) {
	attacher := &subtitleAttachStub{}
	service, dataStore, admin := newSubtitleIndexService(t, attacher)
	runSubtitleIndexJob(t, service, dataStore, admin, "job-attach-subs", "request_attach_subs")
	if len(attacher.ids) != 1 || attacher.ids[0] != "emby-item" {
		t.Fatalf("attached ids=%v", attacher.ids)
	}
	assertTransferEvent(t, service, admin.ID, "job-attach-subs", "已自动挂载中文字幕（1）")
}

func TestMovieIndexRecordsSubtitleAttachFailure(t *testing.T) {
	service, dataStore, admin := newSubtitleIndexService(t, &subtitleAttachStub{
		err: errors.New("Assrt download failed https://example.com/sub?token=secret"),
	})
	runSubtitleIndexJob(t, service, dataStore, admin, "job-attach-fail", "request_attach_fail")
	detail := assertTransferEvent(t, service, admin.ID, "job-attach-fail", "自动挂载中文字幕失败：Assrt download failed")
	for _, event := range detail.Events {
		if strings.Contains(event.Message, "example.com") || strings.Contains(event.Message, "token=") {
			t.Fatalf("event leaked secret: %q", event.Message)
		}
	}
}

func newSubtitleIndexService(t *testing.T, attacher *subtitleAttachStub) (*Service, *store.Store, store.AdminUser) {
	t.Helper()
	ctx := context.Background()
	embyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != "emby-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if request.URL.Path == "/Items" {
			_, _ = w.Write([]byte(`{"Items":[{"Id":"emby-item","Name":"Movie","Type":"Movie","ProductionYear":2026,"ProviderIds":{"Tmdb":"123"}}],"TotalRecordCount":1}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(embyServer.Close)
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dataStore.Close() })
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
	service := NewService(
		dataStore, search.NewService(transferSourceStub{}), codec,
		emby.NewClient(embyServer.URL, "emby-key", time.Second),
		nil,
		config.Workflow{
			Movie: config.WorkflowTarget{DestinationID: "100", QMediaSyncTargetPath: "/media/电影", EmbyLibraryID: "library-movies"},
		},
		nil, nil, nil,
	)
	service.UseSubtitles(attacher)
	return service, dataStore, admin
}

func runSubtitleIndexJob(t *testing.T, service *Service, dataStore *store.Store, admin store.AdminUser, jobID, idempotency string) {
	t.Helper()
	ctx := context.Background()
	job := store.TransferJob{
		ID: jobID, UserID: admin.ID, IdempotencyKey: idempotency,
		RequestHash: []byte("hash"), SelectionToken: "encrypted", SourceID: "sidhub",
		CandidateID: "candidate", Title: "Movie", Year: 2026, MediaType: "movie", TMDBID: "123",
		State: "queued", CreatedAt: 1, UpdatedAt: 1,
	}
	if _, _, err := dataStore.CreateTransferJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	job.State = "indexing_emby"
	job.UpdatedAt = 2
	if updated, err := dataStore.UpdateTransferJob(ctx, job, "queued", "setup index"); err != nil || !updated {
		t.Fatalf("prepare index job: updated=%v err=%v", updated, err)
	}
	loaded, err := dataStore.TransferJob(ctx, admin.ID, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.processJob(ctx, loaded); err != nil {
		t.Fatal(err)
	}
}

type countingDownloadStub struct{ calls *int }

func (countingDownloadStub) ID() string    { return "moviepilot" }
func (countingDownloadStub) Label() string { return "PT" }
func (countingDownloadStub) Search(context.Context, string) ([]search.Candidate, error) {
	return []search.Candidate{{
		ID: "item-1", Title: "Movie", MediaType: "movie", TMDBID: "123", SourceRef: "9d7e672:1", TransferState: "downloadable",
	}}, nil
}
func (stub countingDownloadStub) StartDownload(context.Context, search.DownloadRequest) error {
	*stub.calls++
	return nil
}

func TestDownloadJobCompletesWithoutSTRM(t *testing.T) {
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
	downloads := 0
	searchService := search.NewService(countingDownloadStub{calls: &downloads})
	service := NewService(dataStore, searchService, codec, nil, nil, config.Workflow{}, nil, nil, nil)
	candidate := searchService.Search(ctx, "Movie").Results[0]
	token := service.SelectionToken(candidate)
	if token == "" {
		t.Fatal("download token was not created")
	}
	publicJob, _, err := service.Enqueue(ctx, admin.ID, token, "request_download")
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
	if job.State != "completed" || downloads != 1 {
		t.Fatalf("state=%q downloads=%d", job.State, downloads)
	}
}

func assertTransferEvent(t *testing.T, service *Service, userID int64, jobID, message string) JobDetail {
	t.Helper()
	detail, err := service.Get(context.Background(), userID, jobID)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range detail.Events {
		if event.Message == message {
			return detail
		}
	}
	t.Fatalf("missing %q in events=%#v", message, detail.Events)
	return detail
}

func TestSanitizeSubtitleErrorStripsURLs(t *testing.T) {
	got := sanitizeSubtitleError(errors.New("download failed https://assrt.net/v1/sub?token=abc pickcode=xyz"))
	if got != "download failed" {
		t.Fatalf("got %q", got)
	}
}
