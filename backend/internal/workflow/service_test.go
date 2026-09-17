package workflow

import (
	"context"
	"encoding/base64"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"media-hub/backend/internal/config"
	"media-hub/backend/internal/emby"
	"media-hub/backend/internal/search"
	"media-hub/backend/internal/selection"
	"media-hub/backend/internal/store"
)

type transferSourceStub struct{}

func (transferSourceStub) ID() string    { return "framehdr" }
func (transferSourceStub) Label() string { return "帧影" }
func (transferSourceStub) Search(context.Context, string) ([]search.Candidate, error) {
	return []search.Candidate{{
		ID: "item-1", Title: "Movie", MediaType: "movie", TMDBID: "123", IdentityVerified: true,
		SourceRef: "private-reference", TransferState: "available",
	}}, nil
}
func (transferSourceStub) StartTransfer(context.Context, search.TransferRequest) (search.TransferResult, error) {
	return search.TransferResult{Status: "completed", FileID: "file-1", Path: "/Movies/Movie", IsFile: false}, nil
}
func (transferSourceStub) TransferStatus(context.Context, int64, string) (search.TransferResult, error) {
	return search.TransferResult{}, nil
}

func TestSelectionTokenAndEnqueueAreIdempotent(t *testing.T) {
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
		dataStore,
		searchService,
		codec,
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

	candidate := searchService.Search(ctx, "Movie").Results[0]
	token := service.SelectionToken(candidate)
	if token == "" {
		t.Fatal("selection token was not created")
	}
	first, created, err := service.Enqueue(ctx, admin.ID, token, "request_one")
	if err != nil || !created || first.State != "queued" {
		t.Fatalf("enqueue: job=%#v created=%v err=%v", first, created, err)
	}
	second, created, err := service.Enqueue(ctx, admin.ID, token, "request_one")
	if err != nil || created || second.ID != first.ID {
		t.Fatalf("repeat enqueue: job=%#v created=%v err=%v", second, created, err)
	}

	searchService.Configure(nil, transferSourceStub{})
	if _, _, err := service.Enqueue(ctx, admin.ID, token, "request_stale"); !errors.Is(err, ErrInvalidSelection) {
		t.Fatalf("stale selection error = %v", err)
	}
}

func TestSelectionTokenAllowsShareWithoutTMDB(t *testing.T) {
	codec, err := selection.NewCodec(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatal(err)
	}
	searchService := search.NewService(transferSourceStub{})
	service := NewService(
		nil,
		searchService,
		codec,
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
	candidate := search.Candidate{
		ID: "framehdr:item-1", Title: "Movie", MediaType: "movie",
		SourceID: "framehdr", SourceRef: "private-reference",
		TransferState: "available", Revision: searchService.CurrentRevision(),
	}
	if service.SelectionToken(candidate) == "" {
		t.Fatal("share without TMDB should still receive a transfer token")
	}
}

type shareImportSourceStub struct{}

func (shareImportSourceStub) ID() string    { return "share" }
func (shareImportSourceStub) Label() string { return "115分享" }
func (shareImportSourceStub) Search(context.Context, string) ([]search.Candidate, error) {
	return nil, nil
}
func (shareImportSourceStub) StartTransfer(context.Context, search.TransferRequest) (search.TransferResult, error) {
	return search.TransferResult{Status: "completed", FileID: "file-1", Path: "SSIS-001", IsFile: false}, nil
}
func (shareImportSourceStub) TransferStatus(context.Context, int64, string) (search.TransferResult, error) {
	return search.TransferResult{}, nil
}

func TestEnqueueShareImportCreatesAdultJob(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dataStore.Close() })
	if _, err := dataStore.EnsureAdmin(ctx, "hash"); err != nil {
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
	searchService := search.NewService(shareImportSourceStub{})
	service := NewService(
		dataStore,
		searchService,
		codec,
		emby.NewClient("http://emby.local", "emby-key", time.Second),
		nil,
		config.Workflow{
			StrmBaseURL:   "https://media.example",
			StrmRootMount: "/media",
			Adult:         config.WorkflowTarget{DestinationID: "300", QMediaSyncTargetPath: "/strm/adult", EmbyLibraryID: "adult"},
		},
		nil,
		nil,
		nil,
	)
	job, created, err := service.EnqueueShareImport(ctx, admin.ID, "SSIS-001", "https://115.com/s/shareABC123?password=ab12", "", "share_job_1")
	if err != nil || !created || job.MediaType != "adult" || job.Source != "share" || job.Title != "SSIS-001" || job.TMDBID != "" {
		t.Fatalf("job=%#v created=%v err=%v", job, created, err)
	}
	urlJob, urlCreated, err := service.EnqueueShareImport(ctx, admin.ID, "", "https://cdn.example.com/clip.mkv", "", "share_job_2")
	if err != nil || !urlCreated || urlJob.MediaType != "adult" || urlJob.Title != "clip" {
		t.Fatalf("url job=%#v created=%v err=%v", urlJob, urlCreated, err)
	}
}

type downloadSourceStub struct{}

func (downloadSourceStub) ID() string    { return "moviepilot" }
func (downloadSourceStub) Label() string { return "PT" }
func (downloadSourceStub) Search(context.Context, string) ([]search.Candidate, error) {
	return []search.Candidate{{
		ID: "item-1", Title: "Movie", MediaType: "movie", TMDBID: "123",
		SourceRef: "9d7e672:1", TransferState: "downloadable",
	}}, nil
}
func (downloadSourceStub) StartDownload(context.Context, search.DownloadRequest) error {
	return nil
}

func TestSelectionTokenAllowsMoviePilotDownloadWithout115Target(t *testing.T) {
	codec, err := selection.NewCodec(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatal(err)
	}
	searchService := search.NewService(downloadSourceStub{})
	service := NewService(nil, searchService, codec, nil, nil, config.Workflow{}, nil, nil, nil)
	candidate := searchService.Search(context.Background(), "Movie").Results[0]
	if candidate.TransferState != "downloadable" {
		t.Fatalf("candidate = %#v", candidate)
	}
	if service.SelectionToken(candidate) == "" {
		t.Fatal("moviepilot download should receive a token without 115 workflow")
	}
}

func TestEnqueueRejectsInvalidSelection(t *testing.T) {
	service := NewService(nil, search.NewService(), nil, nil, nil, config.Workflow{}, nil, nil, nil)
	_, _, err := service.Enqueue(context.Background(), 1, "invalid", "request_one")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("enqueue error = %v", err)
	}
}
