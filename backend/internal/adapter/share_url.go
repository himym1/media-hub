package adapter

import (
	"errors"
	"net/url"
	"path"
	"regexp"
	"strings"
)

var (
	fc2CodePattern = regexp.MustCompile(`(?i)FC2[-_]?PPV[-_]?(\d{5,8})`)
	javCodePattern = regexp.MustCompile(`(?i)(?:^|[^A-Z0-9])([A-Z]{2,6}-\d{2,5})(?:[^A-Z0-9]|$)`)
)

// Parse115ShareURL accepts 115.com / 115cdn.com / anxia.com share links.
func Parse115ShareURL(raw, fallbackCode string) (shareCode, receiveCode string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", errors.New("empty share URL")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + strings.TrimPrefix(raw, "//")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil {
		return "", "", errors.New("invalid share URL")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", "", errors.New("invalid share scheme")
	}
	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "115.com", "www.115.com", "115cdn.com", "www.115cdn.com", "anxia.com", "www.anxia.com":
	default:
		return "", "", errors.New("unsupported share host")
	}
	segments := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(segments) < 2 || segments[0] != "s" {
		return "", "", errors.New("invalid share path")
	}
	shareCode, err = url.PathUnescape(segments[1])
	if err != nil || !frameHDRShareCode.MatchString(shareCode) {
		return "", "", errors.New("invalid share code")
	}
	receiveCode = strings.TrimSpace(parsed.Query().Get("password"))
	if receiveCode == "" {
		receiveCode = strings.TrimSpace(parsed.Query().Get("pwd"))
	}
	if receiveCode == "" {
		receiveCode = strings.TrimSpace(fallbackCode)
	}
	if !frameHDRAccessCode.MatchString(receiveCode) {
		return "", "", errors.New("invalid receive code")
	}
	return shareCode, receiveCode, nil
}

func extractAdultCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if match := fc2CodePattern.FindStringSubmatch(value); len(match) == 2 {
		return "FC2-PPV-" + match[1]
	}
	if match := javCodePattern.FindStringSubmatch(strings.ToUpper(value)); len(match) == 2 {
		return match[1]
	}
	return ""
}

func adultLibraryTitle(preferred string, names ...string) string {
	for _, value := range append([]string{preferred}, names...) {
		if code := extractAdultCode(value); code != "" {
			return code
		}
	}
	if title := storageFolderName(preferred, ""); title != "" && !strings.HasPrefix(title, "115分享") {
		return title
	}
	for _, name := range names {
		base := strings.TrimSuffix(path.Base(strings.TrimSpace(name)), path.Ext(name))
		if title := storageFolderName(base, ""); title != "" {
			return title
		}
	}
	return storageFolderName(preferred, "未命名分享")
}
