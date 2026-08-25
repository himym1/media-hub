package assrt

import "testing"

func TestQueriesPrefersSeriesEpisodeThenFilename(t *testing.T) {
	queries := Queries(SearchTarget{
		Title: "第一集", SeriesName: "信号", OriginalTitle: "Signal",
		FileName: "/media3/115-strm/电视剧/信号/S01E01.strm", Year: 2016, Season: 1, Episode: 1, Type: "Episode",
	})
	if len(queries) != 2 || queries[0].Text != "信号 S01E01" || queries[1].Text != "S01E01" || !queries[1].FileName {
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
