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

func TestSubXCommandRetryabilityControlsBlocking(t *testing.T) {
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
	for _, item := range []SubXCommandJob{
		{ID: "terminal", UserID: 1, OperationID: "framehdr.save", IdempotencyKey: "terminal", RequestHash: []byte("a"), PayloadToken: "encrypted", State: "failed", Retryable: false, CreatedAt: now.Unix(), UpdatedAt: now.Unix()},
		{ID: "retryable", UserID: 1, OperationID: "framehdr.save", IdempotencyKey: "retryable", RequestHash: []byte("b"), PayloadToken: "encrypted", State: "failed", Retryable: true, CreatedAt: now.Unix(), UpdatedAt: now.Unix()},
	} {
		if _, _, err := dataStore.CreateSubXCommand(ctx, item); err != nil {
			t.Fatal(err)
		}
		if _, err := dataStore.database.ExecContext(ctx, `UPDATE subx_command_jobs SET retryable = ? WHERE id = ?`, boolInt(item.Retryable), item.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := dataStore.RetrySubXCommand(ctx, 1, "terminal", now.Add(time.Second)); !errors.Is(err, ErrSubXCommandNotRetryable) {
		t.Fatalf("terminal retry error = %v", err)
	}
	if count, err := dataStore.CountBlockingSubXCommands(ctx, 1); err != nil || count != 1 {
		t.Fatalf("blocking commands = %d, err=%v", count, err)
	}
	if job, err := dataStore.RetrySubXCommand(ctx, 1, "retryable", now.Add(time.Second)); err != nil || job.State != "queued" {
		t.Fatalf("retryable command = %#v, err=%v", job, err)
	}
}
