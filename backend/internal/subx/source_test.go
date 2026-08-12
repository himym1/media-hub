package subx

import (
	"encoding/json"
	"testing"
)

func TestCandidateFromObjectRequiresStructuredSeriesRange(t *testing.T) {
	candidate, ok := candidateFromObject(map[string]any{
		"id": "release-1", "title": "Example", "year": float64(2024),
		"media_type": "series", "source": "juying", "tmdb_id": "123",
		"season": float64(2), "episode_start": float64(3), "episode_end": float64(5),
		"resolution": "2160p", "video_codec": "HEVC", "size_bytes": float64(10 << 30),
		"share_code": "private",
	})
	if !ok {
		t.Fatal("candidate was rejected")
	}
	if candidate.Provider != "juying" || candidate.Season != 2 || candidate.EpisodeStart != 3 || candidate.EpisodeEnd != 5 {
		t.Fatalf("candidate = %#v", candidate)
	}
	if candidate.SourceRef == "" {
		t.Fatal("encrypted selection reference input is empty")
	}

	_, ok = candidateFromObject(map[string]any{
		"id": "release-2", "title": "Example", "media_type": "series", "source": "juying",
		"season": float64(2), "resolution": "1080p", "video_codec": "AVC", "size_bytes": float64(1),
	})
	if ok {
		t.Fatal("series candidate without an episode range was accepted")
	}
}

func TestTransferResultFindsProviderCorrelation(t *testing.T) {
	raw := json.RawMessage(`{"data":{"target_cid":"cid-1","target_path":"/series/example","is_file":false}}`)
	result, ok := transferResult(raw)
	if !ok {
		t.Fatal("transfer result was rejected")
	}
	if result.FileID != "cid-1" || result.Path != "/series/example" || result.IsFile {
		t.Fatalf("result = %#v", result)
	}
}
