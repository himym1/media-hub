package drive115

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

const (
	sampleInitEndpoint = "https://uplb.115.com/3.0/sampleinitupload.php"
	maxSampleUpload    = 5 * 1024 * 1024 * 1024
)

// SampleUpload 是短时 OSS PostObject 凭证。文件字节由桌面直传 OSS，不经过 Hub。
type SampleUpload struct {
	Host      string
	Object    string
	AccessID  string
	Policy    string
	Signature string
	Callback  string
	Target    string
	Filename  string
}

func (c *Client) InitSampleUpload(ctx context.Context, destinationID, filename string, size int64) (SampleUpload, error) {
	cookie := c.session()
	if cookie == "" {
		return SampleUpload{}, ErrNotConfigured
	}
	destinationID = strings.TrimSpace(destinationID)
	filename = sanitizeUploadFileName(filename)
	if !numericIDPattern.MatchString(destinationID) || destinationID == "0" || filename == "" {
		return SampleUpload{}, &WriteError{Code: "invalid_request", Err: ErrUpstreamResponse}
	}
	if size < 1 || size > maxSampleUpload {
		return SampleUpload{}, &WriteError{Code: "invalid_request", Err: ErrUpstreamResponse}
	}
	userID := shareUserID(cookie)
	if userID == "" {
		return SampleUpload{}, ErrUnauthorized
	}
	target := "U_1_" + destinationID
	endpoint := c.sampleInitURL
	if endpoint == "" {
		endpoint = sampleInitEndpoint
	}
	form := url.Values{
		"userid":   {userID},
		"filename": {filename},
		"filesize": {strconv.FormatInt(size, 10)},
		"target":   {target},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return SampleUpload{}, &WriteError{Code: "invalid_request", Err: err}
	}
	c.applyAuth(request, cookie)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.client.Do(request)
	if err != nil {
		return SampleUpload{}, &WriteError{Uncertain: true, Code: "uncertain_result", Err: fmt.Errorf("init 115 sample upload: %w", err)}
	}
	defer response.Body.Close()
	c.absorbSessionCookies(response)
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return SampleUpload{}, ErrUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return SampleUpload{}, &WriteError{Code: "provider_rejected", Err: ErrUpstreamResponse}
	}
	var payload struct {
		State     json.RawMessage `json:"state"`
		Error     string          `json:"error"`
		Message   string          `json:"message"`
		Host      string          `json:"host"`
		Object    string          `json:"object"`
		AccessID  string          `json:"accessid"`
		Policy    string          `json:"policy"`
		Signature string          `json:"signature"`
		Callback  string          `json:"callback"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return SampleUpload{}, &WriteError{Uncertain: true, Code: "invalid_response", Err: err}
	}
	if sampleUploadFailed(payload.State, payload.Host, payload.Error, payload.Message) {
		return SampleUpload{}, &WriteError{Code: "provider_rejected", Err: ErrUpstreamResponse}
	}
	host, err := sanitizeOSSPostURL(payload.Host)
	if err != nil || payload.Object == "" || payload.AccessID == "" || payload.Policy == "" || payload.Signature == "" || payload.Callback == "" {
		return SampleUpload{}, &WriteError{Uncertain: true, Code: "invalid_response", Err: ErrUpstreamResponse}
	}
	return SampleUpload{
		Host: host, Object: payload.Object, AccessID: payload.AccessID,
		Policy: payload.Policy, Signature: payload.Signature, Callback: payload.Callback,
		Target: target, Filename: filename,
	}, nil
}

func sampleUploadFailed(state json.RawMessage, host, errText, message string) bool {
	if strings.TrimSpace(host) != "" {
		return false
	}
	text := strings.TrimSpace(string(state))
	if text == "false" || text == `"false"` || text == "0" {
		return true
	}
	return strings.TrimSpace(errText) != "" || strings.TrimSpace(message) != ""
}

func sanitizeOSSPostURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrUpstreamResponse
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + strings.TrimPrefix(raw, "//")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" && parsed.Path != "/" {
		return "", ErrUpstreamResponse
	}
	if parsed.Scheme == "http" {
		parsed.Scheme = "https"
		parsed.Host = parsed.Hostname()
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return "", ErrUpstreamResponse
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" || (!strings.HasSuffix(host, ".aliyuncs.com") && !strings.HasSuffix(host, ".aliyuncs.com.cn")) {
		return "", ErrUpstreamResponse
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func sanitizeUploadFileName(name string) string {
	name = path.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return ""
	}
	cleaned := strings.Map(func(character rune) rune {
		switch character {
		case '/', '\\', 0:
			return -1
		default:
			return character
		}
	}, name)
	runes := []rune(cleaned)
	if len(runes) > 200 {
		cleaned = string(runes[:200])
	}
	ext := strings.ToLower(path.Ext(cleaned))
	if _, ok := uploadVideoExt[ext]; !ok {
		return ""
	}
	return strings.TrimSpace(cleaned)
}

var uploadVideoExt = map[string]struct{}{
	".mp4": {}, ".mkv": {}, ".avi": {}, ".ts": {}, ".m2ts": {}, ".mts": {},
	".wmv": {}, ".flv": {}, ".mov": {}, ".m4v": {}, ".webm": {}, ".mpeg": {},
	".mpg": {}, ".vob": {}, ".f4v": {}, ".asf": {}, ".3gp": {},
}
