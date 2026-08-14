package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestTransferJobCreationIsIdempotentAndDetectsConflicts(t *testing.T) {
	ctx := context.Background()
	dataStore, err := Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()

	if _, err := dataStore.EnsureAdmin(ctx, "password-hash"); err != nil {
		t.Fatal(err)
	}
	admin, exists, err := dataStore.Admin(ctx)
	if err != nil || !exists {
		t.Fatalf("read admin: exists=%v err=%v", exists, err)
	}
	job := TransferJob{
		ID: "job-one", UserID: admin.ID, IdempotencyKey: "request_one",
		RequestHash: []byte("same-hash"), SelectionToken: "encrypted-token",
		SourceID: "frame", CandidateID: "frame:item", Title: "Movie",
		MediaType: "movie", State: "queued", CreatedAt: 100, UpdatedAt: 100,
	}
	createdJob, created, err := dataStore.CreateTransferJob(ctx, job)
	if err != nil || !created || createdJob.ID != job.ID {
		t.Fatalf("create transfer: created=%v job=%#v err=%v", created, createdJob, err)
	}

	duplicate := job
	duplicate.ID = "job-two"
	existing, created, err := dataStore.CreateTransferJob(ctx, duplicate)
	if err != nil || created || existing.ID != job.ID {
		t.Fatalf("repeat transfer: created=%v job=%#v err=%v", created, existing, err)
	}

	duplicate.RequestHash = []byte("different-hash")
	if _, _, err := dataStore.CreateTransferJob(ctx, duplicate); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflicting transfer error = %v", err)
	}
}

func TestInterruptedQMediaSyncSubmissionRequiresAttention(t *testing.T) {
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
	job := TransferJob{
		ID: "job-interrupted", UserID: admin.ID, IdempotencyKey: "request_interrupted",
		RequestHash: []byte("hash"), SelectionToken: "encrypted-token",
		SourceID: "frame", CandidateID: "frame:item", Title: "Movie",
		MediaType: "movie", State: "queued", CreatedAt: 100, UpdatedAt: 100,
	}
	if _, _, err := dataStore.CreateTransferJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	job.State = "submitting_sync"
	job.UpdatedAt = 110
	if updated, err := dataStore.UpdateTransferJob(ctx, job, "queued", "submit"); err != nil || !updated {
		t.Fatalf("transition transfer: updated=%v err=%v", updated, err)
	}

	if err := dataStore.MarkInterruptedSyncSubmissions(ctx, time.Unix(120, 0)); err != nil {
		t.Fatal(err)
	}
	recovered, err := dataStore.TransferJob(ctx, admin.ID, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.State != "needs_attention" || !recovered.Retryable || recovered.ResumeState != "transferred" {
		t.Fatalf("unexpected recovered job: %#v", recovered)
	}
}

func TestRetryTransferUsesPersistedResumeStateAndRecoversDamagedSourceJob(t *testing.T) {
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
	tests := []struct {
		name           string
		state          string
		resumeState    string
		selectionToken string
		providerToken  string
		errorCode      string
		wantState      string
		wantErr        error
	}{
		{name: "source-unknown", state: "needs_attention", resumeState: "transferring", selectionToken: "encrypted", wantState: "transferring"},
		{name: "sync-unknown", state: "needs_attention", resumeState: "transferred", providerToken: "provider", wantState: "transferred"},
		{name: "legacy-attention", state: "needs_attention", selectionToken: "encrypted", wantState: "transferred"},
		{name: "damaged-source-retry", state: "failed", selectionToken: "encrypted", errorCode: "provider_state_invalid", wantState: "transferring"},
		{name: "unrecoverable-provider-state", state: "failed", errorCode: "provider_state_invalid", wantErr: ErrTransferNotRetryable},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			job := TransferJob{
				ID: test.name, UserID: admin.ID, IdempotencyKey: "retry_case_" + test.name,
				RequestHash: []byte("hash"), SelectionToken: test.selectionToken,
				SourceID: "juying", CandidateID: "candidate", Title: "Movie", MediaType: "movie", TMDBID: "123",
				State: "queued", CreatedAt: int64(100 + index), UpdatedAt: int64(100 + index),
			}
			if _, _, err := dataStore.CreateTransferJob(ctx, job); err != nil {
				t.Fatal(err)
			}
			job.State = test.state
			job.ResumeState = test.resumeState
			job.ProviderToken = test.providerToken
			job.ErrorCode = test.errorCode
			job.Retryable = test.state == "needs_attention"
			job.UpdatedAt++
			if updated, err := dataStore.UpdateTransferJob(ctx, job, "queued", "prepare retry case"); err != nil || !updated {
				t.Fatalf("prepare retry case: updated=%v err=%v", updated, err)
			}
			retried, err := dataStore.RetryTransferJob(ctx, admin.ID, job.ID, time.Unix(200, 0))
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr == nil && retried.State != test.wantState {
				t.Fatalf("state = %q, want %q", retried.State, test.wantState)
			}
		})
	}
}

func TestInterruptedNotificationIsNotAutomaticallyReplayed(t *testing.T) {
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
	now := time.Unix(1_720_000_000, 0).UTC()
	if _, _, err := dataStore.CreateTransferJob(ctx, TransferJob{
		ID: "job-notify", UserID: admin.ID, IdempotencyKey: "request_notify",
		RequestHash: []byte("hash"), SelectionToken: "encrypted", SourceID: "frame",
		CandidateID: "candidate", Title: "Movie", Year: 2026, MediaType: "movie", TMDBID: "123",
		State: "completed", CreatedAt: now.Unix(), UpdatedAt: now.Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := dataStore.EnsureTerminalNotifications(ctx, now); err != nil {
		t.Fatal(err)
	}
	item, found, err := dataStore.NextTransferNotification(ctx, now)
	if err != nil || !found {
		t.Fatalf("notification found=%v err=%v", found, err)
	}
	if begun, err := dataStore.BeginTransferNotification(ctx, item, now); err != nil || !begun {
		t.Fatalf("notification begun=%v err=%v", begun, err)
	}
	if err := dataStore.MarkInterruptedNotifications(ctx, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, found, err := dataStore.NextTransferNotification(ctx, now.Add(time.Minute)); err != nil || found {
		t.Fatalf("interrupted notification runnable=%v err=%v", found, err)
	}
	items, err := dataStore.ListTransferNotifications(ctx, admin.ID, 10)
	if err != nil || len(items) != 1 || items[0].State != "needs_attention" {
		t.Fatalf("notifications=%#v err=%v", items, err)
	}
	if _, err := dataStore.RetryTransferNotificationByUser(
		ctx, admin.ID, item.JobID, item.EventType, now.Add(2*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	if _, found, err := dataStore.NextTransferNotification(ctx, now.Add(time.Minute)); err != nil || !found {
		t.Fatalf("explicitly retried notification runnable=%v err=%v", found, err)
	}
}
