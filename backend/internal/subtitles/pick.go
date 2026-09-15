package subtitles

import (
	"sort"
	"strings"

	"media-hub/backend/internal/assrt"
	"media-hub/backend/internal/emby"
)

const (
	AttachExisted = "existed"
	AttachAdded   = "attached"
	AttachMissing = "missing"
)

func PickChinese(items []emby.RemoteSubtitle, mediaPath string) (emby.RemoteSubtitle, bool) {
	ranked := rankChinese(items, mediaPath)
	if len(ranked) == 0 {
		return emby.RemoteSubtitle{}, false
	}
	return ranked[0], true
}

func rankChinese(items []emby.RemoteSubtitle, mediaPath string) []emby.RemoteSubtitle {
	type scored struct {
		item  emby.RemoteSubtitle
		score int
	}
	ranked := make([]scored, 0, len(items))
	for _, item := range items {
		score := scoreChinese(item, mediaPath)
		if score < 0 {
			continue
		}
		ranked = append(ranked, scored{item: item, score: score})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})
	out := make([]emby.RemoteSubtitle, len(ranked))
	for i, item := range ranked {
		out[i] = item.item
	}
	return out
}

func scoreChinese(item emby.RemoteSubtitle, mediaPath string) int {
	text := strings.ToLower(strings.TrimSpace(item.Name + " " + item.Comment + " " + item.Language))
	if text == "" {
		return 0
	}
	englishOnly := strings.Contains(text, "english") || strings.Contains(text, "eng")
	chinese := strings.Contains(text, "chi") || strings.Contains(text, "chs") || strings.Contains(text, "cht") ||
		strings.Contains(text, "zh") || strings.Contains(text, "简") || strings.Contains(text, "繁") ||
		strings.Contains(text, "中文") || strings.Contains(text, "中字")
	if englishOnly && !chinese {
		return -1
	}
	score := 1
	if item.IsHashMatch {
		score += 100
	}
	if strings.Contains(text, "简体") || strings.Contains(text, "简中") || strings.Contains(text, "chs") || strings.Contains(text, "zh-cn") {
		score += 20
	}
	if chinese {
		score += 10
	}
	if strings.EqualFold(item.Format, "ass") || strings.EqualFold(item.Format, "ssa") {
		score += 8
	}
	if item.DownloadCount > 0 {
		score += min(item.DownloadCount, 100) / 20
	}
	if mediaPath != "" {
		score += assrt.ScoreReleases(
			assrt.ParseRelease(mediaPath),
			assrt.ParseRelease(item.Name+" "+item.Comment),
		)
	}
	return score
}
