package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestOperationalStatisticsAreScopedToUser(t *testing.T) {
	ctx := context.Background()
	dataStore, err := Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
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
	for _, job := range []TransferJob{
		{ID: "active", UserID: admin.ID, IdempotencyKey: "active-key", RequestHash: []byte("active"), SelectionToken: "encrypted", SourceID: "framehdr", CandidateID: "one", Title: "One", MediaType: "movie", State: "queued", CreatedAt: 1, UpdatedAt: 1},
		{ID: "done", UserID: admin.ID, IdempotencyKey: "done-key", RequestHash: []byte("done"), SelectionToken: "encrypted", SourceID: "juying", CandidateID: "two", Title: "Two", MediaType: "movie", State: "completed", CreatedAt: 2, UpdatedAt: 2},
	} {
		if _, _, err := dataStore.CreateTransferJob(ctx, job); err != nil {
			t.Fatal(err)
		}
	}
	value, err := dataStore.OperationalStatistics(ctx, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if value.TransfersTotal != 2 || value.TransfersActive != 1 || value.TransfersCompleted != 1 {
		t.Fatalf("statistics = %#v", value)
	}
	if _, err := dataStore.SetTransferArchived(ctx, admin.ID, "done", true, time.Unix(3, 0)); err != nil {
		t.Fatal(err)
	}
	value, err = dataStore.OperationalStatistics(ctx, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if value.TransfersTotal != 1 || value.TransfersActive != 1 || value.TransfersCompleted != 0 {
		t.Fatalf("statistics after archive = %#v", value)
	}
}
