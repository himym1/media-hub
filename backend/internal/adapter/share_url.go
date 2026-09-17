package adapter

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"path"
	"regexp"
	"strings"
)

var (
	fc2CodePattern = regexp.MustCompile(`(?i)FC2[-_]?PPV[-_]?(\d{5,8})`)
	javCodePattern = regexp.MustCompile(`(?i)(?:^|[^A-Z0-9])([A-Z]{2,6}-\d{2,5})(?:[^A-Z0-9]|$)`)

	ErrUnsupportedImportURL = errors.New("unsupported import URL")
	ErrOtherCloudImport     = errors.New("other cloud import")
	ErrNeed115ShareOrFile   = errors.New("need 115 share or file URL")
)

const (
	ImportKindShare = "share"
	ImportKindURL   = "url"
	maxImportURLLen = 8192
)

var otherCloudImportHosts = map[string]struct{}{
	"pan.quark.cn": {}, "www.pan.quark.cn": {},
	"aliyundrive.com": {}, "www.aliyundrive.com": {},
	"alipan.com": {}, "www.alipan.com": {},
	"pan.baidu.com": {}, "www.pan.baidu.com": {}, "yun.baidu.com": {},
	"123pan.com": {}, "www.123pan.com": {},
}

var importVideoExt = map[string]struct{}{
	".mp4": {}, ".mkv": {}, ".avi": {}, ".ts": {}, ".m2ts": {}, ".mts": {},
	".wmv": {}, ".flv": {}, ".mov": {}, ".iso": {}, ".rmvb": {}, ".rm": {},
	".m4v": {}, ".webm": {}, ".mpeg": {}, ".mpg": {}, ".vob": {}, ".f4v": {},
	".asf": {}, ".3gp": {},
}

type AdultImport struct {
	Kind        string
	ShareCode   string
	ReceiveCode string
	URL         string
}

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

func ParseAdultImport(raw, fallbackCode string) (AdultImport, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxImportURLLen {
		return AdultImport{}, ErrUnsupportedImportURL
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "magnet:") {
		if !strings.Contains(lower, "xt=urn:btih:") {
			return AdultImport{}, ErrUnsupportedImportURL
		}
		return AdultImport{Kind: ImportKindURL, URL: raw}, nil
	}
	if strings.HasPrefix(lower, "ed2k://") {
		if len(raw) < 16 {
			return AdultImport{}, ErrUnsupportedImportURL
		}
		return AdultImport{Kind: ImportKindURL, URL: raw}, nil
	}
	if shareCode, receiveCode, err := Parse115ShareURL(raw, fallbackCode); err == nil {
		return AdultImport{Kind: ImportKindShare, ShareCode: shareCode, ReceiveCode: receiveCode}, nil
	}
	parsed, err := parseImportHTTPURL(raw)
	if err != nil {
		return AdultImport{}, err
	}
	host := strings.ToLower(parsed.Hostname())
	if _, blocked := otherCloudImportHosts[host]; blocked {
		return AdultImport{}, ErrOtherCloudImport
	}
	if is115ShareHost(host) {
		if looksLikeImportFile(parsed) {
			return AdultImport{Kind: ImportKindURL, URL: parsed.String()}, nil
		}
		return AdultImport{}, ErrNeed115ShareOrFile
	}
	return AdultImport{Kind: ImportKindURL, URL: parsed.String()}, nil
}

func parseImportHTTPURL(raw string) (*url.URL, error) {
	if !strings.Contains(raw, "://") {
		raw = "https://" + strings.TrimPrefix(raw, "//")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.Host == "" {
		return nil, ErrUnsupportedImportURL
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, ErrUnsupportedImportURL
	}
	return parsed, nil
}

func is115ShareHost(host string) bool {
	switch host {
	case "115.com", "www.115.com", "115cdn.com", "www.115cdn.com", "anxia.com", "www.anxia.com":
		return true
	default:
		return false
	}
}

func looksLikeImportFile(parsed *url.URL) bool {
	_, ok := importVideoExt[strings.ToLower(path.Ext(parsed.Path))]
	return ok
}

func ImportCandidateID(value AdultImport) string {
	if value.Kind == ImportKindShare {
		return "share-" + value.ShareCode
	}
	sum := sha256.Sum256([]byte(value.URL))
	return "url-" + hex.EncodeToString(sum[:8])
}

func ImportDefaultTitle(title string, value AdultImport) string {
	title = strings.TrimSpace(title)
	if title != "" {
		return title
	}
	if value.Kind == ImportKindShare {
		return "115分享 " + value.ShareCode
	}
	if name := importURLTitle(value.URL); name != "" && name != "/" && name != "." {
		return name
	}
	return "视频导入"
}

func importURLTitle(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	base := path.Base(parsed.Path)
	base = strings.TrimSuffix(base, path.Ext(base))
	return strings.TrimSpace(base)
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
