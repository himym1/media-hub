package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSourceCheckInClaimFinishAndRetry(t *testing.T) {
	ctx := context.Background()
	dataStore, err := Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	now := time.Unix(1_720_000_000, 0).UTC()
	first, claimed, err := dataStore.ClaimSourceCheckIn(ctx, "framehdr", "帧影", "2026-08-21", now, false)
	if err != nil || !claimed || first.State != "running" {
		t.Fatalf("first claim=%#v claimed=%v err=%v", first, claimed, err)
	}
	again, claimed, err := dataStore.ClaimSourceCheckIn(ctx, "framehdr", "帧影", "2026-08-21", now.Add(time.Second), false)
	if err != nil || claimed || again.State != "running" {
		t.Fatalf("in-flight claim=%#v claimed=%v err=%v", again, claimed, err)
	}
	first.State = "completed"
	first.Message = "今日已签到"
	first.LastDay = "2026-08-21"
	first.LastSuccessAt = now.Unix()
	first.UpdatedAt = now.Unix()
	if err := dataStore.FinishSourceCheckIn(ctx, first); err != nil {
		t.Fatal(err)
	}
	done, claimed, err := dataStore.ClaimSourceCheckIn(ctx, "framehdr", "帧影", "2026-08-21", now.Add(time.Minute), false)
	if err != nil || claimed || done.State != "completed" {
		t.Fatalf("same-day claim=%#v claimed=%v err=%v", done, claimed, err)
	}
	if _, err := dataStore.RetrySourceCheckIn(ctx, "framehdr", now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	retried, claimed, err := dataStore.ClaimSourceCheckIn(ctx, "framehdr", "帧影", "2026-08-21", now.Add(3*time.Minute), false)
	if err != nil || !claimed || retried.State != "running" {
		t.Fatalf("retry claim=%#v claimed=%v err=%v", retried, claimed, err)
	}
}

func TestRecoverInterruptedCheckIns(t *testing.T) {
	ctx := context.Background()
	dataStore, err := Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	now := time.Unix(1_720_000_100, 0).UTC()
	if _, claimed, err := dataStore.ClaimSourceCheckIn(ctx, "juying", "聚影", "2026-08-21", now, false); err != nil || !claimed {
		t.Fatalf("claimed=%v err=%v", claimed, err)
	}
	if err := dataStore.RecoverInterruptedCheckIns(ctx, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	item, err := dataStore.SourceCheckIn(ctx, "juying")
	if err != nil || item.State != "failed" || !item.Retryable || item.ErrorCode != "interrupted" {
		t.Fatalf("recovered=%#v err=%v", item, err)
	}
}
