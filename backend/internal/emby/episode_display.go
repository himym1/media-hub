package emby

import (
	"regexp"
	"sort"
	"strings"
)

func sortEpisodes(items []Episode) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Season != items[j].Season {
			return items[i].Season < items[j].Season
		}
		return items[i].Episode < items[j].Episode
	})
}

var (
	releaseNameToken   = regexp.MustCompile(`(?i)(?:UHDTV|WEB-?DL|WEBRip|Blu-?Ray|HDTV|HEVC|x264|x265|\bAVC\b|10bit|8bit|2160p|1080p|720p|480p|HDR10|\bHDR\b|Dolby|TrueHD|Atmos|\bDTS\b|DD[25]\.[01]|\bAAC\b|\d{2}fps)`)
	releaseYearEpisode = regexp.MustCompile(`(?i)\b(?:19|20)\d{2}\b.*\bE\d{1,3}\b|\bE\d{1,3}\b.*\b(?:19|20)\d{2}\b`)
	genericEpisodeName = regexp.MustCompile(`(?i)^(?:第\s*[0-9一二三四五六七八九十百]+\s*集|(?:episode|ep\.?|e)\s*0*[0-9]+)$`)
)

func presentEpisodeItem(item baseItem) baseItem {
	mediaPath := strings.TrimSpace(item.Path)
	if mediaPath == "" && len(item.MediaSources) > 0 {
		mediaPath = strings.TrimSpace(item.MediaSources[0].Path)
	}
	season, episode, _ := normalizeEpisodeNumbering(item.Type, item.ParentIndexNumber, item.IndexNumber, mediaPath)
	if parsedSeason, parsedEpisode, parsed := parseSeasonEpisode(item.Name); parsed {
		if yearLikeSeason(season) && parsedSeason > 0 && !yearLikeSeason(parsedSeason) {
			season = parsedSeason
		}
		if episode <= 0 && parsedEpisode > 0 {
			episode = parsedEpisode
		}
	}
	item.ParentIndexNumber = season
	item.IndexNumber = episode
	item.Name = episodeDisplayName(item.Name, item.SeriesName)
	return item
}

func episodeDisplayName(name, seriesName string) string {
	name = strings.TrimSpace(name)
	if name == "" || looksLikeReleaseName(name) || isGenericEpisodeName(name) {
		return ""
	}
	if seriesName = strings.TrimSpace(seriesName); seriesName != "" && strings.EqualFold(name, seriesName) {
		return ""
	}
	return boundedText(name, 80)
}

func looksLikeReleaseName(name string) bool {
	return releaseNameToken.MatchString(name) || releaseYearEpisode.MatchString(name)
}

func isGenericEpisodeName(name string) bool {
	return genericEpisodeName.MatchString(strings.TrimSpace(name))
}
