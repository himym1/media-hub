package subxmigration

import (
	"encoding/json"
	"testing"
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
