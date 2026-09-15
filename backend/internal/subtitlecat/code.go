package subtitlecat

import (
	"path/filepath"
	"regexp"
	"strings"
)

var studioCode = regexp.MustCompile(`(?i)\b([A-Z]{2,8})[-_ ]?(\d{3,4})\b`)

var studioDenied = map[string]struct{}{
	"WEB": {}, "HDR": {}, "AVC": {}, "AAC": {}, "MP4": {}, "MKV": {}, "MOV": {},
	"DTS": {}, "EAC": {}, "AC3": {}, "FLAC": {}, "UHD": {}, "REMUX": {}, "BLURAY": {},
	"HEVC": {}, "X264": {}, "X265": {}, "H264": {}, "H265": {}, "HDR10": {},
}

func SearchQuery(name, originalTitle, fileName string) string {
	if code := ExtractStudioCode(name, originalTitle, fileName); code != "" {
		return code
	}
	base := strings.TrimSpace(name)
	if base == "" {
		base = strings.TrimSuffix(filepath.Base(strings.ReplaceAll(fileName, "\\", "/")), filepath.Ext(fileName))
	}
	base = strings.Join(strings.Fields(base), " ")
	if len([]rune(base)) < 2 {
		return ""
	}
	return base
}

func ExtractStudioCode(values ...string) string {
	for _, value := range values {
		for _, match := range studioCode.FindAllStringSubmatch(value, -1) {
			prefix := strings.ToUpper(strings.TrimSpace(match[1]))
			number := strings.TrimSpace(match[2])
			if _, denied := studioDenied[prefix]; denied {
				continue
			}
			if len(prefix) < 3 && !strings.Contains(match[0], "-") {
				continue
			}
			return prefix + "-" + number
		}
	}
	return ""
}
