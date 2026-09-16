package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestSubscriptionPersistenceAndScheduling(t *testing.T) {
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
	item := Subscription{
		ID: "subscription-1", UserID: 1, TMDBID: "1396", Title: "Breaking Bad",
		MediaType: "series", Season: 1, Policy: "once", Enabled: true,
		IntervalMinutes: 60, SourceIDsJSON: `[]`, PreferencesJSON: `{}`,
		NextRunAt: now.Unix(), CreatedAt: now.Unix(), UpdatedAt: now.Unix(),
	}
	if _, err := dataStore.CreateSubscription(ctx, item); err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	item.ID = "subscription-duplicate"
	if _, err := dataStore.CreateSubscription(ctx, item); !errors.Is(err, ErrSubscriptionConflict) {
		t.Fatalf("duplicate error = %v, want ErrSubscriptionConflict", err)
	}

	claimed, run, found, err := dataStore.ClaimDueSubscription(ctx, "run-1", now)
	if err != nil || !found {
		t.Fatalf("claim due subscription: found=%v err=%v", found, err)
	}
	if run.SubscriptionID != "subscription-1" || run.State != "queued" {
		t.Fatalf("unexpected run: %#v", run)
	}
	if claimed.NextRunAt != now.Add(time.Hour).Unix() {
		t.Fatalf("next run = %d, want %d", claimed.NextRunAt, now.Add(time.Hour).Unix())
	}

	if _, _, found, err := dataStore.ClaimDueSubscription(ctx, "run-2", now.Add(time.Hour)); err != nil || !found {
		t.Fatalf("defer active subscription: found=%v err=%v", found, err)
	}
	runs, err := dataStore.SubscriptionRuns(ctx, 1, "subscription-1", 10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("run count = %d, want 1", len(runs))
	}
}

func TestManualSubscriptionRunRejectsConcurrentRun(t *testing.T) {
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
	_, err = dataStore.CreateSubscription(ctx, Subscription{
		ID: "subscription-1", UserID: 1, TMDBID: "7131", Title: "Van Helsing",
		MediaType: "movie", Policy: "once", Enabled: false, IntervalMinutes: 60,
		SourceIDsJSON: `[]`, PreferencesJSON: `{}`, CreatedAt: now.Unix(), UpdatedAt: now.Unix(),
	})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	if _, err := dataStore.CreateManualSubscriptionRun(ctx, 1, "subscription-1", "run-1", now); err != nil {
		t.Fatalf("create manual run: %v", err)
	}
	if _, err := dataStore.CreateManualSubscriptionRun(ctx, 1, "subscription-1", "run-2", now); !errors.Is(err, ErrSubscriptionRunActive) {
		t.Fatalf("second run error = %v, want ErrSubscriptionRunActive", err)
	}
}

func TestFailedSubscriptionCandidateIsNotTriedAgain(t *testing.T) {
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
	_, err = dataStore.CreateSubscription(ctx, Subscription{
		ID: "subscription-1", UserID: 1, TMDBID: "396535", Title: "釜山行",
		MediaType: "movie", Policy: "once", Enabled: true, IntervalMinutes: 60,
		SourceIDsJSON: `[]`, PreferencesJSON: `{}`, CreatedAt: now.Unix(), UpdatedAt: now.Unix(),
	})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	run, err := dataStore.CreateManualSubscriptionRun(ctx, 1, "subscription-1", "run-1", now)
	if err != nil {
		t.Fatalf("create manual run: %v", err)
	}
	run.State = "failed"
	run.CandidateFingerprint = "framehdr:dead-share"
	run.FinishedAt = now.Unix()
	run.UpdatedAt = now.Unix()
	if err := dataStore.UpdateSubscriptionRun(ctx, run, "queued"); err != nil {
		t.Fatalf("mark run failed: %v", err)
	}
	seen, err := dataStore.HasSubscriptionCandidate(ctx, "subscription-1", "framehdr:dead-share")
	if err != nil || !seen {
		t.Fatalf("failed fingerprint seen=%v err=%v", seen, err)
	}
	seen, err = dataStore.HasSubscriptionCandidate(ctx, "subscription-1", "moviepilot:other")
	if err != nil || seen {
		t.Fatalf("other fingerprint seen=%v err=%v", seen, err)
	}
}
