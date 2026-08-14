package drive115

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var offlineTimePattern = regexp.MustCompile(`^[0-9]{1,20}$`)

func (c *Client) AddOfflineURLs(ctx context.Context, destinationID string, urls []string) error {
	if len(urls) == 0 {
		return &WriteError{Code: "invalid_request", Err: ErrUpstreamResponse}
	}
	for _, item := range urls {
		if strings.TrimSpace(item) == "" {
			return &WriteError{Code: "invalid_request", Err: ErrUpstreamResponse}
		}
	}
	if destinationID == "" {
		destinationID = "0"
	}
	cookie := c.session()
	if cookie == "" {
		return ErrNotConfigured
	}

	sign, timestamp, err := c.offlineSignature(ctx, cookie)
	if err != nil {
		return err
	}
	form := url.Values{
		"wp_path_id": {destinationID},
		"sign":       {sign},
		"time":       {timestamp},
	}
	for index, item := range urls {
		form[fmt.Sprintf("url[%d]", index)] = []string{item}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.offlineAddURL, strings.NewReader(form.Encode()))
	if err != nil {
		return &WriteError{Code: "invalid_request", Err: err}
	}
	c.applyAuth(request, cookie)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.client.Do(request)
	if err != nil {
		return &WriteError{Uncertain: true, Code: "uncertain_result", Err: fmt.Errorf("submit 115 offline task: %w", err)}
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &WriteError{Code: "provider_rejected", Err: ErrUpstreamResponse}
	}
	var payload struct {
		State  bool `json:"state"`
		Result []struct {
			InfoHash string `json:"info_hash"`
		} `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return &WriteError{Uncertain: true, Code: "invalid_response", Err: err}
	}
	if !payload.State {
		return &WriteError{Code: "provider_rejected", Err: ErrUpstreamResponse}
	}
	if len(payload.Result) != len(urls) {
		return &WriteError{Uncertain: true, Code: "partial_result", Err: ErrUpstreamResponse}
	}
	for _, item := range payload.Result {
		if strings.TrimSpace(item.InfoHash) == "" {
			return &WriteError{Uncertain: true, Code: "partial_result", Err: ErrUpstreamResponse}
		}
	}
	return nil
}

func (c *Client) offlineSignature(ctx context.Context, cookie string) (string, string, error) {
	var payload struct {
		Sign string          `json:"sign"`
		Time json.RawMessage `json:"time"`
	}
	if err := c.getJSONWithSession(ctx, c.offlineInfoURL, nil, cookie, &payload); err != nil {
		return "", "", err
	}
	sign := strings.TrimSpace(payload.Sign)
	timestamp := strings.Trim(strings.TrimSpace(string(payload.Time)), `"`)
	if sign == "" || len(sign) > 4096 || !offlineTimePattern.MatchString(timestamp) {
		return "", "", ErrUpstreamResponse
	}
	return sign, timestamp, nil
}

func (s *AuthService) AddOfflineURLs(ctx context.Context, destinationID string, urls []string) error {
	if err := s.prepareSession(ctx); err != nil {
		return err
	}
	return s.drive.AddOfflineURLs(ctx, destinationID, urls)
}
