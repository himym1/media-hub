package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestSubXCommandIdempotencyAndInterruptedRecovery(t *testing.T) {
	ctx := context.Background()
	dataStore, err := Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer dataStore.Close()
	if _, err := dataStore.EnsureAdmin(ctx, "encoded-password-hash"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	now := time.Unix(1_720_000_000, 0).UTC()
	input := SubXCommandJob{
		ID: "command-1", UserID: 1, OperationID: "drive115.sign", IdempotencyKey: "idempotency-1",
		RequestHash: []byte("request-a"), PayloadToken: "encrypted", State: "queued",
		CreatedAt: now.Unix(), UpdatedAt: now.Unix(),
	}
	created, isNew, err := dataStore.CreateSubXCommand(ctx, input)
	if err != nil || !isNew || created.ID != input.ID {
		t.Fatalf("create command: job=%#v new=%v err=%v", created, isNew, err)
	}
	replayed, isNew, err := dataStore.CreateSubXCommand(ctx, SubXCommandJob{
		ID: "command-2", UserID: 1, OperationID: input.OperationID, IdempotencyKey: input.IdempotencyKey,
		RequestHash: input.RequestHash, PayloadToken: "different-ciphertext", State: "queued",
		CreatedAt: now.Unix(), UpdatedAt: now.Unix(),
	})
	if err != nil || isNew || replayed.ID != input.ID {
		t.Fatalf("replay command: job=%#v new=%v err=%v", replayed, isNew, err)
	}
	_, _, err = dataStore.CreateSubXCommand(ctx, SubXCommandJob{
		ID: "command-3", UserID: 1, OperationID: input.OperationID, IdempotencyKey: input.IdempotencyKey,
		RequestHash: []byte("request-b"), PayloadToken: "encrypted", State: "queued",
		CreatedAt: now.Unix(), UpdatedAt: now.Unix(),
	})
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflict error = %v, want ErrIdempotencyConflict", err)
	}

	begun, ok, err := dataStore.BeginSubXCommand(ctx, input.ID, now.Add(time.Second))
	if err != nil || !ok || begun.State != "submitting" {
		t.Fatalf("begin command: job=%#v ok=%v err=%v", begun, ok, err)
	}
	if err := dataStore.MarkInterruptedSubXCommands(ctx, now.Add(2*time.Second)); err != nil {
		t.Fatalf("recover commands: %v", err)
	}
	recovered, err := dataStore.SubXCommand(ctx, 1, input.ID)
	if err != nil {
		t.Fatalf("read recovered command: %v", err)
	}
	if recovered.State != "needs_attention" || recovered.Retryable {
		t.Fatalf("recovered command = %#v", recovered)
	}
	count, err := dataStore.CountBlockingSubXCommands(ctx, 1)
	if err != nil || count != 1 {
		t.Fatalf("blocking commands = %d, err=%v", count, err)
	}
}
