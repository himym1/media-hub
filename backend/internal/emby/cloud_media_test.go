package emby

import "testing"

func TestPickCodeFromValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"query", "http://qms.local/115/url/video.mkv?pickcode=Abcd1234", "Abcd1234"},
		{"pc query", "http://qms.local/115/url?pc=Abcd1234", "Abcd1234"},
		{"scheme", "115://Abcd1234", "Abcd1234"},
		{"strm path", "/library/Movie.strm", ""},
		{"bare code", "abcd1234", "abcd1234"},
		{"https cdn", "https://cdn.example/movie.mkv?token=1", ""},
		{"local mkv", "/volume1/media/movie.mkv", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := pickCodeFromValue(test.value); got != test.want {
				t.Fatalf("got %q want %q", got, test.want)
			}
		})
	}
}

func TestPlaybackURLFromValueAccepts115CDNOnly(t *testing.T) {
	if got := playbackURLFromValue("https://cdnfhnfile.115.com/video.mkv?t=1"); got == "" {
		t.Fatal("expected 115 CDN URL")
	}
	if got := playbackURLFromValue("https://115.com/s/abc?password=1"); got != "" {
		t.Fatalf("share page should be rejected: %q", got)
	}
	if got := playbackURLFromValue("https://other.example/video.mkv"); got != "" {
		t.Fatalf("foreign host should be rejected: %q", got)
	}
}

func TestFilter115ItemsKeepsCloudMovies(t *testing.T) {
	items := filter115Items([]baseItem{
		{ID: "local", Name: "Local", Type: "Movie", Path: "/volume1/media/movie.mkv"},
		{ID: "foreign", Name: "Foreign", Type: "Movie", Path: "/library/other.strm", MediaSources: []mediaSource{{Path: "https://other.example/video.m3u8"}}},
		{ID: "cloud", Name: "Cloud", Type: "Movie", Path: "/library/movie.strm"},
		{ID: "pick", Name: "Pick", Type: "Movie", Path: "http://qms.local/115/url/video.mkv?pickcode=abcd1234"},
		{ID: "series-1", Name: "Series", Type: "Series"},
	}, map[string]struct{}{"series-1": {}})
	if len(items) != 3 || items[0].ID != "cloud" || items[1].ID != "pick" || items[2].ID != "series-1" {
		t.Fatalf("items=%#v", items)
	}
}
