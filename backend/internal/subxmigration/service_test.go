package subxmigration

import (
	"encoding/json"
	"testing"

	"media-hub/backend/internal/integration"
	"media-hub/backend/internal/subscription"
)

func TestExtractSubscriptionsExpandsSeasonsAndRejectsIncompleteSeries(t *testing.T) {
	raw := json.RawMessage(`{
		"contents": [
			{"tmdb_id": 123, "title": "Example", "media_type": "tv", "year": 2024, "seasons": [1, 2]},
			{"tmdb_id": 456, "title": "Movie", "media_type": "movie", "year": 2023},
			{"tmdb_id": 789, "title": "Incomplete", "media_type": "series"}
		]
	}`)
	items, detected, rejected := extractSubscriptions(raw)
	if detected != 3 || rejected != 1 || len(items) != 3 {
		t.Fatalf("detected=%d rejected=%d items=%#v", detected, rejected, items)
	}
	if items[0].Season != 1 || items[1].Season != 2 || items[2].MediaType != "movie" {
		t.Fatalf("unexpected expansion: %#v", items)
	}
	for _, item := range items {
		if len(item.SourceIDs) == 0 {
			t.Fatalf("missing native source mapping: %#v", item.SourceIDs)
		}
		for _, sourceID := range item.SourceIDs {
			if sourceID == "subx" {
				t.Fatalf("migration retained SubX source: %#v", item.SourceIDs)
			}
		}
	}
}

func TestReadinessFailsClosedUntilEveryReplacementGatePasses(t *testing.T) {
	healthy := []integration.Health{
		{ID: "tmdb", Status: integration.StatusHealthy},
		{ID: "115", Status: integration.StatusHealthy},
		{ID: "qmediasync", Status: integration.StatusHealthy},
		{ID: "emby", Status: integration.StatusHealthy},
		{ID: "sources", Status: integration.StatusHealthy},
	}
	ready := evaluateReadiness(Readiness{
		CoreConfigurationReady: true, NativeSourceCount: 1, ParallelValidationCompleted: true, DelegatedGroups: []string{},
	}, 0, healthy)
	if !ready.CanStopSubX || len(ready.Blockers) != 0 {
		t.Fatalf("ready = %#v", ready)
	}

	notValidated := evaluateReadiness(Readiness{
		CoreConfigurationReady: true, NativeSourceCount: 1, DelegatedGroups: []string{},
	}, 0, healthy)
	if notValidated.CanStopSubX || len(notValidated.Blockers) != 1 {
		t.Fatalf("not validated = %#v", notValidated)
	}

	unhealthy := append([]integration.Health(nil), healthy...)
	unhealthy[1].Status = integration.StatusUnavailable
	providerDown := evaluateReadiness(Readiness{
		CoreConfigurationReady: true, NativeSourceCount: 1, ParallelValidationCompleted: true, DelegatedGroups: []string{},
	}, 0, unhealthy)
	if providerDown.CanStopSubX || len(providerDown.Blockers) != 1 {
		t.Fatalf("provider down = %#v", providerDown)
	}

	empty := evaluateReadiness(Readiness{DelegatedGroups: []string{}}, 2, nil)
	if empty.CanStopSubX || empty.DelegatedOperations != 2 || len(empty.Blockers) < 8 {
		t.Fatalf("empty = %#v", empty)
	}
}

func TestFallbackSubscriptionsRemainShutdownBlockers(t *testing.T) {
	items := []subscription.Subscription{
		{SourceIDs: []string{"subx"}},
		{Preferences: subscription.Preferences{PreferredSources: []string{"subx"}}},
		{},
	}
	if got := countFallbackSubscriptions(items); got != 2 {
		t.Fatalf("fallback subscriptions = %d", got)
	}
	result := evaluateReadiness(Readiness{FallbackSubscriptions: 2, DelegatedGroups: []string{}}, 0, nil)
	if result.CanStopSubX || result.DelegatedOperations != 2 || !containsString(result.DelegatedGroups, "fallback-subscriptions") {
		t.Fatalf("readiness = %#v", result)
	}
}
