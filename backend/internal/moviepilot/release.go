package moviepilot

import (
	"regexp"
	"strconv"
	"strings"

	"media-hub/backend/internal/search"
)

var (
	resolutionPattern = regexp.MustCompile(`(?i)\b(2160p|1080p|720p|480p|4k)\b`)
	codecPattern      = regexp.MustCompile(`(?i)\b(HEVC|x265|H\.?265|AV1|AVC|x264|H\.?264)\b`)
	hdrPattern        = regexp.MustCompile(`(?i)\b(Dolby[ ._-]?Vision|DV|HDR10\+?|HDR|HLG)\b`)
	audioPattern      = regexp.MustCompile(`(?i)\b(TrueHD|Atmos|DTS-HD|DTS|DDP|DD\+|EAC3|AC3|AAC|FLAC)\b`)
	sizePattern       = regexp.MustCompile(`(?i)^\s*([0-9]+(?:\.[0-9]+)?)\s*([KMGT])(?:I?B)?\s*$`)
)

func releaseFacts(title, size string) search.ReleaseFacts {
	resolution := strings.ToLower(resolutionPattern.FindString(title))
	if resolution == "4k" {
		resolution = "2160p"
	}
	return search.ReleaseFacts{
		Resolution:   resolution,
		VideoCodec:   normalizedCodec(title),
		DynamicRange: normalizedHDR(title),
		Audio:        audioPattern.FindString(title),
		SizeBytes:    parseSizeBytes(size),
	}
}

func normalizedCodec(title string) string {
	value := strings.ToLower(strings.NewReplacer(".", "", " ", "").Replace(codecPattern.FindString(title)))
	switch value {
	case "hevc", "x265", "h265":
		return "HEVC"
	case "av1":
		return "AV1"
	case "avc", "x264", "h264":
		return "AVC"
	default:
		return ""
	}
}

func normalizedHDR(title string) string {
	value := strings.ToLower(hdrPattern.FindString(title))
	switch {
	case strings.Contains(value, "vision") || value == "dv":
		return "Dolby Vision"
	case strings.Contains(value, "hdr10+"):
		return "HDR10+"
	case strings.Contains(value, "hdr"):
		return "HDR10"
	case value == "hlg":
		return "HLG"
	default:
		return ""
	}
}

func parseSizeBytes(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if parsed, err := strconv.ParseInt(value, 10, 64); err == nil && parsed > 0 {
		return parsed
	}
	match := sizePattern.FindStringSubmatch(value)
	if len(match) != 3 {
		return 0
	}
	amount, err := strconv.ParseFloat(match[1], 64)
	if err != nil || amount <= 0 {
		return 0
	}
	multiplier := int64(1000)
	switch strings.ToUpper(match[2]) {
	case "K":
		multiplier = 1000
	case "M":
		multiplier = 1000 * 1000
	case "G":
		multiplier = 1000 * 1000 * 1000
	case "T":
		multiplier = 1000 * 1000 * 1000 * 1000
	}
	return int64(amount * float64(multiplier))
}
