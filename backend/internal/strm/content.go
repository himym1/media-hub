package strm

import (
	"net/url"
	"path/filepath"
	"strings"
)

var videoExtensions = map[string]struct{}{
	".mkv": {}, ".mp4": {}, ".avi": {}, ".mov": {}, ".wmv": {},
	".flv": {}, ".webm": {}, ".m4v": {}, ".3gp": {}, ".ts": {},
}

func VideoExtension(name string) (string, bool) {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(name)))
	_, ok := videoExtensions[ext]
	return ext, ok
}

func SubtitleExtension(name string) (string, bool) {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(name)))
	switch ext {
	case ".ass", ".ssa", ".srt":
		return ext, true
	default:
		return "", false
	}
}

func URL(baseURL, ext, pickCode, userID string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	query := url.Values{}
	query.Set("pickcode", strings.TrimSpace(pickCode))
	query.Set("userid", strings.TrimSpace(userID))
	return base + "/115/url/video" + strings.ToLower(ext) + "?" + query.Encode()
}
