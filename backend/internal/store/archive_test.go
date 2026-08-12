package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestArchivePlanConfirmationAndInterruptedRecovery(t *testing.T) {
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
	input := ArchivePlan{ID: "arc-test", UserID: 1, PayloadToken: "encrypted", State: "awaiting_confirmation", StepTotal: 2, CreatedAt: now.Unix(), UpdatedAt: now.Unix()}
	if err := dataStore.CreateArchivePlan(ctx, input); err != nil {
		t.Fatal(err)
	}
	if _, err := dataStore.ConfirmArchivePlan(ctx, 1, input.ID, "wrong", now); !errors.Is(err, ErrArchivePlanNotFound) {
		t.Fatalf("confirmation error = %v", err)
	}
	confirmed, err := dataStore.ConfirmArchivePlan(ctx, 1, input.ID, input.ID, now)
	if err != nil || confirmed.State != "queued" {
		t.Fatalf("confirmed = %#v, err=%v", confirmed, err)
	}
	begun, ok, err := dataStore.BeginArchivePlan(ctx, input.ID, now)
	if err != nil || !ok || begun.State != "running" {
		t.Fatalf("begun = %#v, ok=%v, err=%v", begun, ok, err)
	}
	if err := dataStore.AdvanceArchivePlan(ctx, input.ID, 1, now); err != nil {
		t.Fatal(err)
	}
	if err := dataStore.MarkInterruptedArchivePlans(ctx, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	recovered, err := dataStore.ArchivePlan(ctx, 1, input.ID)
	if err != nil || recovered.State != "needs_attention" || recovered.StepIndex != 1 {
		t.Fatalf("recovered = %#v, err=%v", recovered, err)
	}
}
