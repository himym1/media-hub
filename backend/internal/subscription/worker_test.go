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
	}, nil)
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
	}, nil)
	if !found || selected.ID != "matching" {
		t.Fatalf("selected = %#v found=%v", selected, found)
	}
}

func TestSearchQueriesUsesOriginalTitleAndYear(t *testing.T) {
	got := searchQueries(store.Subscription{
		Title: "毒液", OriginalTitle: "Venom", Year: 2018, Season: 0,
	})
	want := []string{"毒液", "Venom", "毒液 2018", "Venom 2018"}
	if len(got) != len(want) {
		t.Fatalf("queries = %#v", got)
	}
	for index, query := range want {
		if got[index] != query {
			t.Fatalf("queries = %#v", got)
		}
	}
	series := searchQueries(store.Subscription{
		Title: "信号", OriginalTitle: "Signal", Year: 2016, Season: 1,
	})
	if series[0] != "信号 S1" || series[1] != "Signal S1" || series[2] != "信号 S1 2016" || series[3] != "Signal S1 2016" {
		t.Fatalf("series queries = %#v", series)
	}
}

func TestSelectCandidateSkipsFailedShareFingerprints(t *testing.T) {
	item := store.Subscription{TMDBID: "396535", MediaType: "movie", Year: 2016}
	dead := search.Candidate{
		ID: "dead-share", SourceID: "framehdr", TMDBID: "396535", MediaType: "movie", Year: 2016,
		IdentityVerified: true,
		Release:          search.ReleaseFacts{Resolution: "2160p", VideoCodec: "HEVC", SizeBytes: 20 << 30},
	}
	other := search.Candidate{
		ID: "moviepilot", SourceID: "moviepilot", TMDBID: "396535", MediaType: "movie", Year: 2016,
		IdentityVerified: true,
		Release:          search.ReleaseFacts{Resolution: "1080p", VideoCodec: "AVC", SizeBytes: 8 << 30},
	}
	skip := map[string]struct{}{candidateFingerprint(dead): {}}
	selected, found := selectCandidate([]search.Candidate{dead, other}, item, nil, Preferences{}, skip)
	if !found || selected.ID != "moviepilot" {
		t.Fatalf("selected = %#v found=%v", selected, found)
	}
}
