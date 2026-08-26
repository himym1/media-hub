package assrt

import (
	"regexp"
	"strings"
	"unicode"
)

type ReleaseProfile struct {
	Sources     map[string]struct{}
	Resolutions map[string]struct{}
	Groups      map[string]struct{}
}

var (
	bracketGroupPattern  = regexp.MustCompile(`\[([^\[\]]{2,40})]`)
	trailingGroupPattern = regexp.MustCompile(`[-.]([A-Za-z][A-Za-z0-9]{2,20})$`)
)

var sourceAlias = map[string]string{
	"webdl": "webdl", "web-dl": "webdl", "web.dl": "webdl", "web_dl": "webdl",
	"webrip": "webrip", "web-rip": "webrip", "web.rip": "webrip",
	"bluray": "bluray", "blu-ray": "bluray", "blu.ray": "bluray", "bdrip": "bluray", "bd-rip": "bluray",
	"hdtv": "hdtv", "uhdtv": "uhdtv", "remux": "remux",
}

var resolutionAlias = map[string]string{
	"2160p": "2160p", "4k": "2160p", "1080p": "1080p", "720p": "720p", "480p": "480p",
}

func ParseRelease(name string) ReleaseProfile {
	profile := ReleaseProfile{
		Sources:     map[string]struct{}{},
		Resolutions: map[string]struct{}{},
		Groups:      map[string]struct{}{},
	}
	stem := fileStem(name)
	lower := strings.ToLower(stem)
	for raw, canon := range sourceAlias {
		if containsReleaseToken(lower, raw) {
			profile.Sources[canon] = struct{}{}
		}
	}
	for raw, canon := range resolutionAlias {
		if containsReleaseToken(lower, raw) {
			profile.Resolutions[canon] = struct{}{}
		}
	}
	for _, match := range bracketGroupPattern.FindAllStringSubmatch(stem, 4) {
		addReleaseGroup(profile.Groups, match[1])
	}
	if match := trailingGroupPattern.FindStringSubmatch(stem); len(match) == 2 {
		addReleaseGroup(profile.Groups, match[1])
	}
	return profile
}

func ScoreHit(hit Hit, target SearchTarget) int {
	return ScoreReleases(
		ParseRelease(target.FileName),
		ParseRelease(strings.Join([]string{hit.Name, hit.VideoName, hit.Comment}, " ")),
	)
}

func ScoreReleases(resource, subtitle ReleaseProfile) int {
	score := 0
	if overlap(resource.Groups, subtitle.Groups) {
		score += 40
	}
	if overlap(resource.Sources, subtitle.Sources) {
		score += 25
	} else if conflict(resource.Sources, subtitle.Sources) {
		score -= 15
	}
	if overlap(resource.Resolutions, subtitle.Resolutions) {
		score += 10
	} else if conflict(resource.Resolutions, subtitle.Resolutions) {
		score -= 5
	}
	return score
}

func addReleaseGroup(groups map[string]struct{}, raw string) {
	normalized := strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(raw))), " ")
	if normalized == "" || strings.Contains(normalized, ".") || qualityToken[strings.ReplaceAll(normalized, " ", "")] {
		return
	}
	if _, isSource := sourceAlias[strings.ReplaceAll(normalized, " ", "")]; isSource {
		return
	}
	if _, isResolution := resolutionAlias[normalized]; isResolution {
		return
	}
	letters := 0
	for _, r := range normalized {
		if unicode.IsLetter(r) {
			letters++
		}
	}
	if letters < 3 {
		return
	}
	groups[normalized] = struct{}{}
}

func overlap(left, right map[string]struct{}) bool {
	for key := range left {
		if _, ok := right[key]; ok {
			return true
		}
	}
	return false
}

func conflict(left, right map[string]struct{}) bool {
	return len(left) > 0 && len(right) > 0 && !overlap(left, right)
}

func containsReleaseToken(haystack, token string) bool {
	if token == "" {
		return false
	}
	start := 0
	for {
		index := strings.Index(haystack[start:], token)
		if index < 0 {
			return false
		}
		index += start
		leftOK := index == 0 || !unicode.IsLetter(rune(haystack[index-1])) && !unicode.IsDigit(rune(haystack[index-1]))
		right := index + len(token)
		rightOK := right == len(haystack) || !unicode.IsLetter(rune(haystack[right])) && !unicode.IsDigit(rune(haystack[right]))
		if leftOK && rightOK {
			return true
		}
		start = index + 1
	}
}
