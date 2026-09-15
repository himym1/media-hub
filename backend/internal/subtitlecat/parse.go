package subtitlecat

import (
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
)

var (
	searchHref = regexp.MustCompile(`(?i)href="(/?subs/[^"]+\.html)"`)
	fileHref   = regexp.MustCompile(`(?i)href="(/?subs/[^"]+\.(?:srt|ass|ssa))"`)
)

func parseSearchPages(baseURL string, body []byte) []string {
	seen := make(map[string]struct{})
	pages := make([]string, 0, 4)
	for _, match := range searchHref.FindAllSubmatch(body, -1) {
		resolved := resolveURL(baseURL, string(match[1]))
		if resolved == "" {
			continue
		}
		if _, exists := seen[resolved]; exists {
			continue
		}
		seen[resolved] = struct{}{}
		pages = append(pages, resolved)
		if len(pages) >= 4 {
			break
		}
	}
	return pages
}

func parseDetailHits(baseURL, pageURL string, body []byte) []Hit {
	hits := make([]Hit, 0, 8)
	seen := make(map[string]struct{})
	for _, match := range fileHref.FindAllSubmatch(body, -1) {
		fileURL := resolveURL(baseURL, string(match[1]))
		if fileURL == "" {
			continue
		}
		if _, exists := seen[fileURL]; exists {
			continue
		}
		language, ok := languageFromURL(fileURL)
		if !ok {
			continue
		}
		seen[fileURL] = struct{}{}
		name := path.Base(fileURL)
		if decoded, err := url.PathUnescape(name); err == nil {
			name = decoded
		}
		hits = append(hits, Hit{
			ID:       encodeID(fileURL),
			Name:     name,
			Language: language,
			Format:   fileFormat(name),
			PageURL:  pageURL,
			FileURL:  fileURL,
		})
	}
	return hits
}

func languageFromURL(fileURL string) (string, bool) {
	lower := strings.ToLower(fileURL)
	switch {
	case strings.Contains(lower, "-zh-cn.") || strings.Contains(lower, "_zh-cn.") || strings.Contains(lower, "-zh_cn."):
		return "zh-CN", true
	case strings.Contains(lower, "-zh-tw.") || strings.Contains(lower, "_zh-tw.") || strings.Contains(lower, "-zh_tw."):
		return "zh-TW", true
	case strings.Contains(lower, "-zh.") || strings.HasSuffix(lower, "-zh.srt"):
		return "zh", true
	case strings.Contains(lower, "-ja.") || strings.Contains(lower, "-jp."):
		return "ja", true
	case strings.Contains(lower, "-en."):
		return "en", true
	default:
		return "", false
	}
}

func fileFormat(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".ass":
		return "ass"
	case ".ssa":
		return "ssa"
	default:
		return "srt"
	}
}

func languageRank(language string) int {
	switch strings.ToLower(language) {
	case "zh-cn", "zh":
		return 0
	case "zh-tw":
		return 1
	case "ja":
		return 2
	case "en":
		return 3
	default:
		return 9
	}
}

func sortHits(hits []Hit) {
	sort.SliceStable(hits, func(i, j int) bool {
		return languageRank(hits[i].Language) < languageRank(hits[j].Language)
	})
}

func resolveURL(baseURL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	base, err := url.Parse(baseURL + "/")
	if err != nil {
		return ""
	}
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return base.ResolveReference(ref).String()
}
