package emby

import "testing"

func TestEpisodeDisplayName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		seriesName string
		want       string
	}{
		{name: "Show 2016 E02 UHDTV HEVC 10bit 60fps DD2.0-Group", want: ""},
		{name: "Show.S01E03.1080p.WEB-DL.x265", want: ""},
		{name: "第二集", want: ""},
		{name: "Episode 4", want: ""},
		{name: "E05", want: ""},
		{name: "验收剧集", seriesName: "验收剧集", want: ""},
		{name: "连接", seriesName: "验收剧集", want: "连接"},
		{name: "The Dinner Party", want: "The Dinner Party"},
	}
	for _, tc := range cases {
		if got := episodeDisplayName(tc.name, tc.seriesName); got != tc.want {
			t.Fatalf("episodeDisplayName(%q, %q) = %q, want %q", tc.name, tc.seriesName, got, tc.want)
		}
	}
}

func TestPresentEpisodeItemNormalizesYearSeason(t *testing.T) {
	t.Parallel()
	item := presentEpisodeItem(baseItem{
		ID:                "episode-2",
		Name:              "Show 2016 E02 UHDTV HEVC 10bit 60fps DD2.0-Group",
		Type:              "Episode",
		ParentIndexNumber: 2016,
		IndexNumber:       2,
		SeriesName:        "验收剧集",
		Path:              "/library/Show 2016 E02 UHDTV HEVC 10bit.strm",
	})
	if item.ParentIndexNumber != 1 || item.IndexNumber != 2 || item.Name != "" {
		t.Fatalf("present = season %d episode %d name %q", item.ParentIndexNumber, item.IndexNumber, item.Name)
	}
}

func TestSortEpisodesBySeasonThenIndex(t *testing.T) {
	t.Parallel()
	items := []Episode{
		{Item: Item{ID: "e2", Season: 1, Episode: 2}},
		{Item: Item{ID: "e1", Season: 1, Episode: 1}},
		{Item: Item{ID: "s2e1", Season: 2, Episode: 1}},
	}
	sortEpisodes(items)
	if items[0].ID != "e1" || items[1].ID != "e2" || items[2].ID != "s2e1" {
		t.Fatalf("order = %#v", items)
	}
}
