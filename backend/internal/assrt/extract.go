package assrt

import (
	"archive/zip"
	"bytes"
	"io"
	"path"
	"strings"
)

func extractSubtitle(name string, body []byte, hint FileHint) (string, []byte, error) {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".ass"), strings.HasSuffix(lower, ".ssa"), strings.HasSuffix(lower, ".srt"):
		if looksLikeSubtitle(body) {
			return path.Base(name), body, nil
		}
		if looksLikeZip(body) {
			return extractZip(body, hint)
		}
		return path.Base(name), body, nil
	case strings.HasSuffix(lower, ".zip") || looksLikeZip(body):
		return extractZip(body, hint)
	case strings.HasSuffix(lower, ".rar"), strings.HasSuffix(lower, ".7z"):
		return "", nil, ErrUnsupportedFile
	default:
		if looksLikeZip(body) {
			return extractZip(body, hint)
		}
		if looksLikeSubtitle(body) {
			if ext := subtitleExtension(name); ext != "" {
				return path.Base(name), body, nil
			}
			return "subtitle.srt", body, nil
		}
		return "", nil, ErrUnsupportedFile
	}
}

func extractZip(body []byte, hint FileHint) (string, []byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return "", nil, ErrUnsupportedFile
	}
	bestName, bestScore := "", -1
	var best *zip.File
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		name := file.Name
		if strings.Contains(name, "__MACOSX") {
			continue
		}
		score := subtitleFileScore(name, hint)
		if score > bestScore {
			bestScore = score
			bestName = name
			best = file
		}
	}
	if best == nil || bestScore < 50 {
		return "", nil, ErrUnsupportedFile
	}
	opened, err := best.Open()
	if err != nil {
		return "", nil, err
	}
	defer opened.Close()
	data, err := io.ReadAll(io.LimitReader(opened, 4<<20+1))
	if err != nil {
		return "", nil, err
	}
	if len(data) > 4<<20 {
		return "", nil, ErrUpstreamResponse
	}
	return path.Base(bestName), data, nil
}

func looksLikeZip(body []byte) bool {
	return len(body) >= 4 && body[0] == 'P' && body[1] == 'K'
}

func looksLikeSubtitle(body []byte) bool {
	text := string(body[:min(len(body), 512)])
	return strings.Contains(text, "Dialogue:") || strings.Contains(text, "-->") || strings.Contains(strings.ToLower(text), "[script info]")
}

func subtitleExtension(name string) string {
	lower := strings.ToLower(name)
	for _, ext := range []string{".ass", ".ssa", ".srt"} {
		if strings.HasSuffix(lower, ext) {
			return ext
		}
	}
	return ""
}
