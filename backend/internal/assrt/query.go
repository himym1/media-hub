package assrt

import (
	"fmt"
	"path"
	"strings"
)

type Query struct {
	Text     string
	FileName bool
}

type SearchTarget struct {
	Title         string
	SeriesName    string
	OriginalTitle string
	FileName      string
	Year          int
	Season        int
	Episode       int
	Type          string
}

func Queries(target SearchTarget) []Query {
	seen := make(map[string]struct{})
	queries := make([]Query, 0, 3)
	add := func(text string, fileName bool) {
		text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
		if len([]rune(text)) < 3 {
			return
		}
		key := strings.ToLower(text) + "|" + fmt.Sprint(fileName)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		queries = append(queries, Query{Text: text, FileName: fileName})
	}

	episode := seasonEpisode(target.Season, target.Episode)
	isEpisode := strings.EqualFold(target.Type, "Episode") || target.Episode > 0
	if isEpisode && episode != "" {
		if series := firstNonEmpty(target.SeriesName, target.Title); series != "" {
			add(series+" "+episode, false)
		}
	} else {
		title := firstNonEmpty(target.Title, target.SeriesName)
		if title != "" && target.Year > 0 {
			add(fmt.Sprintf("%s %d", title, target.Year), false)
		} else {
			add(title, false)
		}
	}
	if fileName := fileStem(target.FileName); fileName != "" {
		add(fileName, true)
	}
	if isEpisode && episode != "" {
		if original := strings.TrimSpace(target.OriginalTitle); original != "" && !sameTitle(original, target.SeriesName) && !sameTitle(original, target.Title) {
			add(original+" "+episode, false)
		}
	} else {
		title := firstNonEmpty(target.Title, target.SeriesName)
		if original := strings.TrimSpace(target.OriginalTitle); original != "" && !sameTitle(original, title) {
			if target.Year > 0 {
				add(fmt.Sprintf("%s %d", original, target.Year), false)
			} else {
				add(original, false)
			}
		}
	}
	if len(queries) > 2 {
		return queries[:2]
	}
	return queries
}

func seasonEpisode(season, episode int) string {
	if episode <= 0 {
		return ""
	}
	if season <= 0 {
		season = 1
	}
	return fmt.Sprintf("S%02dE%02d", season, episode)
}

func fileStem(name string) string {
	base := path.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	if base == "." || base == "/" || base == "" {
		return ""
	}
	if ext := path.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	return strings.TrimSpace(base)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func sameTitle(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}
