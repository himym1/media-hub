package subscription

import (
	"testing"

	"media-hub/backend/internal/search"
	"media-hub/backend/internal/store"
)

func TestSelectCandidateUsesIdentitySeasonAndPreferenceOrder(t *testing.T) {
	item := store.Subscription{TMDBID: "1396", MediaType: "series", Season: 2, Year: 2009}
	candidates := []search.Candidate{
		{
			ID: "wrong-season", SourceID: "framehdr", TMDBID: "1396", MediaType: "series", Season: 1,
			Year: 2009, IdentityVerified: true,
			Release: search.ReleaseFacts{Resolution: "2160p", VideoCodec: "HEVC", SizeBytes: 20 << 30},
		},
		{
			ID: "1080", SourceID: "juying", TMDBID: "1396", MediaType: "series", Season: 2,
			Year: 2009, IdentityVerified: true,
			Release: search.ReleaseFacts{Resolution: "1080p", VideoCodec: "HEVC", SizeBytes: 10 << 30},
		},
		{
			ID: "2160", SourceID: "framehdr", TMDBID: "1396", MediaType: "series", Season: 2,
			Year: 2009, IdentityVerified: true,
			Release: search.ReleaseFacts{Resolution: "2160p", VideoCodec: "HEVC", SizeBytes: 18 << 30},
		},
	}
	selected, found := selectCandidate(candidates, item, nil, Preferences{
		Resolutions: []string{"2160p", "1080p"},
		VideoCodecs: []string{"HEVC"},
	})
	if !found || selected.ID != "2160" {
		t.Fatalf("selected = %#v found=%v", selected, found)
	}
}

func TestSelectCandidateAppliesSourceAudioAndSizeConstraints(t *testing.T) {
	item := store.Subscription{TMDBID: "7131", MediaType: "movie", Year: 2004}
	candidates := []search.Candidate{
		{
			ID: "too-large", SourceID: "framehdr", TMDBID: "7131", MediaType: "movie", Year: 2004,
			IdentityVerified: true,
			Release:          search.ReleaseFacts{Resolution: "2160p", VideoCodec: "HEVC", Audio: "TrueHD Atmos", SizeBytes: 80 << 30},
		},
		{
			ID: "matching", SourceID: "juying", TMDBID: "7131", MediaType: "movie", Year: 2004,
			IdentityVerified: true,
			Release:          search.ReleaseFacts{Resolution: "2160p", VideoCodec: "HEVC", Audio: "TrueHD Atmos", SizeBytes: 35 << 30},
		},
	}
	selected, found := selectCandidate(candidates, item, []string{"juying"}, Preferences{
		AudioContains: []string{"Atmos"},
		MaxSizeBytes:  40 << 30,
	})
	if !found || selected.ID != "matching" {
		t.Fatalf("selected = %#v found=%v", selected, found)
	}
}
