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
	"media-hub/backend/internal/qms"
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
		qms.NewClient("http://qms.local", "qms-key", time.Second),
		emby.NewClient("http://emby.local", "emby-key", time.Second),
		nil,
		config.Workflow{
			QMediaSyncAccountID: 3,
			Movie:               config.WorkflowTarget{DestinationID: "100", QMediaSyncTargetPath: "/strm/movies", EmbyLibraryID: "library-movies"},
		},
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

func TestEnqueueRejectsInvalidSelection(t *testing.T) {
	service := NewService(nil, search.NewService(), nil, nil, nil, nil, config.Workflow{})
	_, _, err := service.Enqueue(context.Background(), 1, "invalid", "request_one")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("enqueue error = %v", err)
	}
}
