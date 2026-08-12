package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalUploadIdempotencyAndInterruptedRecovery(t *testing.T) {
	ctx := context.Background()
	dataStore, err := Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	if _, err := dataStore.EnsureAdmin(ctx, "encoded-password-hash"); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_720_000_000, 0).UTC()
	input := LocalUploadJob{ID: "upload-1", UserID: 1, IdempotencyKey: "key-1", RequestHash: []byte("a"), PayloadToken: "encrypted", State: "queued", BytesTotal: 100, CreatedAt: now.Unix(), UpdatedAt: now.Unix()}
	created, isNew, err := dataStore.CreateLocalUpload(ctx, input)
	if err != nil || !isNew || created.ID != input.ID {
		t.Fatalf("created=%#v new=%v err=%v", created, isNew, err)
	}
	replayed, isNew, err := dataStore.CreateLocalUpload(ctx, LocalUploadJob{ID: "upload-2", UserID: 1, IdempotencyKey: "key-1", RequestHash: []byte("a"), PayloadToken: "different", State: "queued", BytesTotal: 100, CreatedAt: now.Unix(), UpdatedAt: now.Unix()})
	if err != nil || isNew || replayed.ID != input.ID {
		t.Fatalf("replayed=%#v new=%v err=%v", replayed, isNew, err)
	}
	if _, _, err := dataStore.CreateLocalUpload(ctx, LocalUploadJob{ID: "upload-3", UserID: 1, IdempotencyKey: "key-1", RequestHash: []byte("b"), PayloadToken: "encrypted", State: "queued", CreatedAt: now.Unix(), UpdatedAt: now.Unix()}); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflict error = %v", err)
	}
	if _, ok, err := dataStore.TransitionLocalUpload(ctx, input.ID, "queued", "hashing", "hash", now); err != nil || !ok {
		t.Fatalf("begin: ok=%v err=%v", ok, err)
	}
	if _, ok, err := dataStore.TransitionLocalUpload(ctx, input.ID, "hashing", "submitting_init", "submit", now); err != nil || !ok {
		t.Fatalf("submit: ok=%v err=%v", ok, err)
	}
	if err := dataStore.MarkInterruptedLocalUploads(ctx, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	recovered, err := dataStore.LocalUpload(ctx, 1, input.ID)
	if err != nil || recovered.State != "needs_attention" {
		t.Fatalf("recovered=%#v err=%v", recovered, err)
	}
}
