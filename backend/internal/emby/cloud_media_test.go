package emby

import "testing"

func TestPickCodeFromValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"query", "http://qms.local/115/url/video.mkv?pickcode=Abcd1234", "Abcd1234"},
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

func TestFilter115ItemsKeepsCloudMovies(t *testing.T) {
	items := filter115Items([]baseItem{
		{ID: "local", Name: "Local", Type: "Movie", Path: "/volume1/media/movie.mkv"},
		{ID: "cloud", Name: "Cloud", Type: "Movie", Path: "/library/movie.strm"},
		{ID: "series-1", Name: "Series", Type: "Series"},
	}, map[string]struct{}{"series-1": {}})
	if len(items) != 2 || items[0].ID != "cloud" || items[1].ID != "series-1" {
		t.Fatalf("items=%#v", items)
	}
}
