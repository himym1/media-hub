package assrt

import "testing"

func TestQueriesPrefersSeriesYearWhenFilenameIsGeneric(t *testing.T) {
	queries := Queries(SearchTarget{
		Title: "第一集", SeriesName: "信号", OriginalTitle: "Signal",
		FileName: "/media3/115-strm/电视剧/信号/S01E01.strm", Year: 2016, Season: 1, Episode: 1, Type: "Episode",
	})
	if len(queries) != 2 || queries[0].Text != "信号 2016 S01E01" || queries[1].Text != "信号 S01E01" {
		t.Fatalf("queries = %#v", queries)
	}
}

func TestQueriesPrefersReleaseFilename(t *testing.T) {
	queries := Queries(SearchTarget{
		Title: "第一集", SeriesName: "信号", OriginalTitle: "Signal",
		FileName: "/media/电视剧/信号 (2016)/Signal 2016 E01 UHDTV HEVC 10bit 60fps DD2.0-HThoreau.strm",
		Year:     2016, Season: 1, Episode: 1, Type: "Episode",
	})
	if len(queries) != 2 || queries[0].Text != "Signal 2016 E01 UHDTV HEVC 10bit 60fps DD2.0-HThoreau" || !queries[0].FileName {
		t.Fatalf("queries = %#v", queries)
	}
	if queries[1].Text != "信号 2016 S01E01" || queries[1].FileName {
		t.Fatalf("queries = %#v", queries)
	}
}

func TestQueriesUsesMovieTitleAndYear(t *testing.T) {
	queries := Queries(SearchTarget{Title: "头号玩家", Year: 2018, Type: "Movie", FileName: "/media/电影/Ready.Player.One.2018.strm"})
	if len(queries) != 2 || queries[0].Text != "头号玩家 2018" || queries[1].Text != "Ready.Player.One.2018" {
		t.Fatalf("queries = %#v", queries)
	}
}

func TestQueriesSkipsShortTitles(t *testing.T) {
	if queries := Queries(SearchTarget{Title: "ab", Type: "Movie"}); len(queries) != 0 {
		t.Fatalf("queries = %#v", queries)
	}
}

func TestRelevantKeepsSignalDropsCrown(t *testing.T) {
	target := SearchTarget{
		Title: "第一集", SeriesName: "信号", OriginalTitle: "Signal",
		FileName: "Signal 2016 E01 UHDTV HEVC 10bit 60fps DD2.0-HThoreau.strm",
		Year:     2016, Season: 1, Episode: 1, Type: "Episode",
	}
	if Relevant(Hit{Name: "【王冠 The.Crown】S01E01.中英双语特效字幕"}, target) {
		t.Fatal("The Crown should not match 信号")
	}
	if !Relevant(Hit{Name: "信号/Signal/信号 시그널 简体修正乱码", VideoName: "시그널"}, target) {
		t.Fatal("Signal 2016 pack should match")
	}
}
