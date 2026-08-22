package mediaidentity

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	seasonEpisode = regexp.MustCompile(`(?i)\bS([0-9]{1,2})E([0-9]{1,4})(?:\s*[-~]\s*E?([0-9]{1,4}))?\b`)
	dotEpisode    = regexp.MustCompile(`(?i)(?:^|[^A-Z0-9])\.?E([0-9]{1,4})(?:\.|[^0-9]|$)`)
	epPrefix      = regexp.MustCompile(`(?i)\bEP([0-9]{1,4})\b`)
	chineseEpisode = regexp.MustCompile(`第([0-9]{1,4})集`)
	videoExtension = regexp.MustCompile(`(?i)\.(mkv|mp4|avi|ts|m2ts|wmv|mov|rmvb|flv|webm)$`)
)

// EpisodeRangeFromText parses season and episode numbers from a release title or file name.
// When only a bare E## marker is found, season defaults to 1.
func EpisodeRangeFromText(text string) (season, start, end int, ok bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, 0, 0, false
	}
	if matched := seasonEpisode.FindStringSubmatch(text); len(matched) == 4 {
		season, _ = strconv.Atoi(matched[1])
		start, _ = strconv.Atoi(matched[2])
		end = start
		if matched[3] != "" {
			end, _ = strconv.Atoi(matched[3])
		}
		if validEpisodeRange(season, start, end) {
			return season, start, end, true
		}
		return 0, 0, 0, false
	}
	for _, pattern := range []*regexp.Regexp{dotEpisode, epPrefix, chineseEpisode} {
		if matched := pattern.FindStringSubmatch(text); len(matched) == 2 {
			start, _ = strconv.Atoi(matched[1])
			if start > 0 && start <= 10000 {
				return 1, start, start, true
			}
		}
	}
	return 0, 0, 0, false
}

// ApplyReleaseIdentity upgrades movie candidates to series when release text contains episode markers.
func ApplyReleaseIdentity(mediaType, releaseTitle string) (string, int, int, int) {
	season, start, end, ok := EpisodeRangeFromText(releaseTitle)
	if !ok {
		if mediaType == "series" {
			return mediaType, 0, 0, 0
		}
		return mediaType, 0, 0, 0
	}
	if mediaType != "series" {
		mediaType = "series"
	}
	if season == 0 {
		season = 1
	}
	if end == 0 {
		end = start
	}
	return mediaType, season, start, end
}

// EpisodeNumber extracts a single episode number from a file or folder name.
func EpisodeNumber(name string) (int, bool) {
	_, start, _, ok := EpisodeRangeFromText(name)
	return start, ok
}

// DistinctEpisodeCount counts unique episode numbers across file names.
func DistinctEpisodeCount(names []string) int {
	seen := make(map[int]struct{})
	for _, name := range names {
		if episode, ok := EpisodeNumber(name); ok {
			seen[episode] = struct{}{}
		}
	}
	return len(seen)
}

// VideoFileCount returns how many names look like video files.
func VideoFileCount(names []string) int {
	count := 0
	for _, name := range names {
		if videoExtension.MatchString(strings.TrimSpace(name)) {
			count++
		}
	}
	return count
}

// MovieShareLooksLikeSeries reports whether transferred share content should be treated as TV episodes.
func MovieShareLooksLikeSeries(names []string) bool {
	if len(names) == 0 {
		return false
	}
	distinct := DistinctEpisodeCount(names)
	if distinct >= 2 {
		return true
	}
	videos := VideoFileCount(names)
	return videos >= 2 && distinct >= 1
}

// ShareContentConflictsWithMediaType returns true when share contents do not match the requested media type.
func ShareContentConflictsWithMediaType(mediaType string, names []string) bool {
	if mediaType == "series" {
		return false
	}
	return MovieShareLooksLikeSeries(names)
}

func validEpisodeRange(season, start, end int) bool {
	return season > 0 && season <= 100 && start > 0 && end >= start && end <= 10000
}
