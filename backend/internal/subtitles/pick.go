package subtitles

import (
	"strings"

	"media-hub/backend/internal/emby"
)

const (
	AttachExisted = "existed"
	AttachAdded   = "attached"
	AttachMissing = "missing"
)

func PickChinese(items []emby.RemoteSubtitle) (emby.RemoteSubtitle, bool) {
	bestScore := -1
	var best emby.RemoteSubtitle
	found := false
	for _, item := range items {
		score := scoreChinese(item)
		if score < 0 {
			continue
		}
		if !found || score > bestScore {
			best, bestScore, found = item, score, true
		}
	}
	return best, found
}

func scoreChinese(item emby.RemoteSubtitle) int {
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
	return score
}
