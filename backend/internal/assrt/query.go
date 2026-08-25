package assrt

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"unicode"
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

var (
	seasonEpisodeToken  = regexp.MustCompile(`(?i)^s\d{1,2}e\d{1,4}$`)
	bareEpisodeToken    = regexp.MustCompile(`(?i)^e\d{1,4}$`)
	chineseEpisodeTitle = regexp.MustCompile(`^第[0-9一二三四五六七八九十百]+集$`)
)

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
	series := firstNonEmpty(target.SeriesName, target.Title)
	original := strings.TrimSpace(target.OriginalTitle)
	stem := fileStem(target.FileName)
	if series != "" && original != "" && !titlesEqual(original, series) && !titlesEqual(original, target.Title) {
		add(series+" "+original, false)
	} else if shortCJKTitle(series) {
		if partner := firstDistinctiveFileToken(stem); partner != "" && !titlesEqual(partner, series) {
			add(series+" "+partner, false)
		}
	}
	if isEpisode && episode != "" && richFileStem(stem) {
		add(stem, true)
	}
	if isEpisode && episode != "" {
		if series != "" {
			if shortCJKTitle(series) {
				if target.Year > 0 {
					add(fmt.Sprintf("%s %d", series, target.Year), false)
				}
			} else {
				if target.Year > 0 {
					add(fmt.Sprintf("%s %d %s", series, target.Year, episode), false)
				}
				add(series+" "+episode, false)
			}
		}
	} else {
		title := firstNonEmpty(target.Title, target.SeriesName)
		if title != "" && target.Year > 0 {
			add(fmt.Sprintf("%s %d", title, target.Year), false)
		} else {
			add(title, false)
		}
	}
	if stem != "" {
		add(stem, true)
	}
	if isEpisode && episode != "" {
		if original != "" && !titlesEqual(original, target.SeriesName) && !titlesEqual(original, target.Title) {
			if target.Year > 0 {
				add(fmt.Sprintf("%s %d %s", original, target.Year, episode), false)
			} else {
				add(original+" "+episode, false)
			}
		}
	} else {
		title := firstNonEmpty(target.Title, target.SeriesName)
		if original != "" && !titlesEqual(original, title) {
			if target.Year > 0 {
				add(fmt.Sprintf("%s %d", original, target.Year), false)
			} else {
				add(original, false)
			}
		}
	}
	if isEpisode && episode != "" && len(queries) == 0 && series != "" {
		add(series+" "+episode, false)
	}
	if len(queries) > 2 {
		return queries[:2]
	}
	return queries
}

func Relevant(hit Hit, target SearchTarget) bool {
	tokens := matchTokens(target)
	if len(tokens) == 0 {
		return true
	}
	haystack := strings.ToLower(strings.TrimSpace(hit.Name) + " " + strings.TrimSpace(hit.VideoName))
	if haystack == "" {
		return false
	}
	matched := false
	for _, token := range tokens {
		if strings.Contains(haystack, token) {
			matched = true
			break
		}
	}
	return matched && !conflictsYear(haystack, target.Year)
}

var foreignYear = regexp.MustCompile(`(?:19|20)\d{2}`)

func conflictsYear(haystack string, year int) bool {
	if year < 1 {
		return false
	}
	want := fmt.Sprintf("%d", year)
	for _, found := range foreignYear.FindAllString(haystack, -1) {
		if found != want {
			return true
		}
	}
	return false
}

func shortCJKTitle(title string) bool {
	runes := []rune(strings.TrimSpace(title))
	if len(runes) == 0 || len(runes) > 2 {
		return false
	}
	for _, r := range runes {
		if !unicode.Is(unicode.Han, r) && !unicode.In(r, unicode.Hangul, unicode.Hiragana, unicode.Katakana) {
			return false
		}
	}
	return true
}

func matchTokens(target SearchTarget) []string {
	seen := make(map[string]struct{})
	tokens := make([]string, 0, 8)
	add := func(value string) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		if !distinctiveToken(value) {
			return
		}
		seen[value] = struct{}{}
		tokens = append(tokens, value)
	}
	add(target.SeriesName)
	if original := strings.TrimSpace(target.OriginalTitle); original != "" {
		add(original)
	}
	if title := strings.TrimSpace(target.Title); title != "" && !genericEpisodeTitle(title) {
		add(title)
	}
	for _, token := range fileTitleTokens(target.FileName) {
		add(token)
	}
	return tokens
}

func firstDistinctiveFileToken(stem string) string {
	for _, token := range strings.FieldsFunc(stem, splitFileToken) {
		if distinctiveToken(token) {
			return token
		}
	}
	return ""
}

func fileTitleTokens(name string) []string {
	tokens := make([]string, 0, 4)
	for _, token := range strings.FieldsFunc(fileStem(name), splitFileToken) {
		if distinctiveToken(token) {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func richFileStem(stem string) bool {
	for _, token := range strings.FieldsFunc(stem, splitFileToken) {
		if distinctiveToken(token) {
			return true
		}
	}
	return false
}

func distinctiveToken(token string) bool {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" || qualityToken[token] {
		return false
	}
	if seasonEpisodeToken.MatchString(token) || bareEpisodeToken.MatchString(token) {
		return false
	}
	letters := 0
	cjk := 0
	digits := 0
	for _, r := range token {
		switch {
		case unicode.Is(unicode.Han, r) || unicode.In(r, unicode.Hangul, unicode.Hiragana, unicode.Katakana):
			cjk++
		case unicode.IsLetter(r):
			letters++
		case unicode.IsDigit(r):
			digits++
		}
	}
	if cjk >= 2 {
		return true
	}
	if letters >= 3 {
		return true
	}
	return false
}

func genericEpisodeTitle(title string) bool {
	title = strings.Join(strings.Fields(strings.TrimSpace(title)), " ")
	if title == "" {
		return true
	}
	lower := strings.ToLower(title)
	if seasonEpisodeToken.MatchString(lower) || bareEpisodeToken.MatchString(lower) {
		return true
	}
	if chineseEpisodeTitle.MatchString(title) {
		return true
	}
	return strings.EqualFold(title, "episode") || strings.HasPrefix(lower, "episode ")
}

func splitFileToken(r rune) bool {
	return r == '.' || r == '_' || r == '-' || r == '[' || r == ']' || r == '(' || r == ')' || unicode.IsSpace(r)
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

func titlesEqual(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

var qualityToken = map[string]bool{
	"10bit": true, "8bit": true, "2160p": true, "1080p": true, "720p": true, "480p": true,
	"4k": true, "60fps": true, "24fps": true, "30fps": true, "aac": true, "ac3": true,
	"bluray": true, "complete": true, "dd2": true, "dd5": true, "dts": true, "end": true,
	"hdr": true, "hevc": true, "hdtv": true, "internal": true, "mkv": true, "mp4": true,
	"proper": true, "remux": true, "strm": true, "uhdtv": true, "web": true, "webdl": true,
	"web-dl": true, "webrip": true, "x264": true, "x265": true,
}
