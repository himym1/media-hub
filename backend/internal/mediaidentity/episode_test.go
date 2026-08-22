package mediaidentity

import "testing"

func TestEpisodeRangeFromText(t *testing.T) {
	tests := []struct {
		text             string
		season, start, end int
		ok               bool
	}{
		{"Van.Helsing.2004.2160p", 0, 0, 0, false},
		{"Show.S01E11-13.1080p", 1, 11, 13, true},
		{"시그널.E11.2160p.END", 1, 11, 11, true},
		{"Signal.E12.WEB-DL", 1, 12, 12, true},
		{"Title.EP03.720p", 1, 3, 3, true},
		{"信号 第11集 1080p", 1, 11, 11, true},
	}
	for _, test := range tests {
		season, start, end, ok := EpisodeRangeFromText(test.text)
		if season != test.season || start != test.start || end != test.end || ok != test.ok {
			t.Fatalf("EpisodeRangeFromText(%q) = (%d,%d,%d,%v), want (%d,%d,%d,%v)",
				test.text, season, start, end, ok, test.season, test.start, test.end, test.ok)
		}
	}
}

func TestApplyReleaseIdentityUpgradesMovieToSeries(t *testing.T) {
	mediaType, season, start, end := ApplyReleaseIdentity("movie", "시그널.E11.2160p")
	if mediaType != "series" || season != 1 || start != 11 || end != 11 {
		t.Fatalf("identity = %s %d %d %d", mediaType, season, start, end)
	}
}

func TestMovieShareLooksLikeSeries(t *testing.T) {
	names := []string{
		"시그널.E11.2160p.mkv",
		"시그널.E12.2160p.mkv",
		"시그널.E13.2160p.mkv",
	}
	if !MovieShareLooksLikeSeries(names) {
		t.Fatal("expected episode pack to be detected")
	}
	if MovieShareLooksLikeSeries([]string{"Van.Helsing.2004.2160p.mkv"}) {
		t.Fatal("single movie file should not be flagged")
	}
}

func TestShareContentConflictsWithMediaType(t *testing.T) {
	names := []string{"시그널.E11.mkv", "시그널.E12.mkv"}
	if !ShareContentConflictsWithMediaType("movie", names) {
		t.Fatal("movie job should conflict with episode pack")
	}
	if ShareContentConflictsWithMediaType("series", names) {
		t.Fatal("series job should accept episode pack")
	}
}
