package drive115

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/SheltonZhu/115driver/pkg/crypto/m115"
)

const offlineAppVersion = "27.0.5.7"

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

	userID, err := c.offlineUserID(ctx, cookie)
	if err != nil {
		return err
	}
	payload, err := offlineRequestPayload(userID, destinationID, urls)
	if err != nil {
		return &WriteError{Code: "invalid_request", Err: err}
	}
	key := m115.GenerateKey()
	form := url.Values{"data": {m115.Encode(payload, key)}}
	endpoint, err := url.Parse(c.offlineAddURL)
	if err != nil {
		return &WriteError{Code: "invalid_request", Err: err}
	}
	query := endpoint.Query()
	query.Set("t", strconv.FormatInt(time.Now().Unix(), 10))
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), strings.NewReader(form.Encode()))
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
	var envelope struct {
		State bool   `json:"state"`
		Data  string `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&envelope); err != nil {
		return &WriteError{Uncertain: true, Code: "invalid_response", Err: err}
	}
	if !envelope.State {
		return &WriteError{Code: "provider_rejected", Err: ErrUpstreamResponse}
	}
	decoded, err := decodeOfflineResponse(envelope.Data, key)
	if err != nil {
		return &WriteError{Uncertain: true, Code: "invalid_response", Err: err}
	}
	var result struct {
		State  bool `json:"state"`
		Result []struct {
			InfoHash string `json:"info_hash"`
		} `json:"result"`
	}
	if err := json.Unmarshal(decoded, &result); err != nil {
		return &WriteError{Uncertain: true, Code: "invalid_response", Err: err}
	}
	if !result.State {
		return &WriteError{Code: "provider_rejected", Err: ErrUpstreamResponse}
	}
	if len(result.Result) != len(urls) {
		return &WriteError{Uncertain: true, Code: "partial_result", Err: ErrUpstreamResponse}
	}
	return nil
}

func (c *Client) offlineUserID(ctx context.Context, cookie string) (int64, error) {
	var payload struct {
		State bool `json:"state"`
		Data  struct {
			UserID int64 `json:"user_id"`
		} `json:"data"`
	}
	if err := c.getJSONWithSession(ctx, c.userProfileURL, nil, cookie, &payload); err != nil {
		return 0, err
	}
	if !payload.State || payload.Data.UserID <= 0 {
		return 0, ErrUnauthorized
	}
	return payload.Data.UserID, nil
}

func offlineRequestPayload(userID int64, destinationID string, urls []string) ([]byte, error) {
	params := map[string]string{
		"ac":         "add_task_urls",
		"wp_path_id": destinationID,
		"app_ver":    offlineAppVersion,
		"uid":        strconv.FormatInt(userID, 10),
	}
	for index, item := range urls {
		params["url["+strconv.Itoa(index)+"]"] = item
	}
	return json.Marshal(params)
}

func decodeOfflineResponse(input string, key m115.Key) (output []byte, err error) {
	defer func() {
		if recover() != nil {
			output = nil
			err = ErrUpstreamResponse
		}
	}()
	output, err = m115.Decode(input, key)
	if err != nil || len(output) == 0 {
		return nil, ErrUpstreamResponse
	}
	return output, nil
}

func (s *AuthService) AddOfflineURLs(ctx context.Context, destinationID string, urls []string) error {
	if err := s.prepareSession(ctx); err != nil {
		return err
	}
	return s.drive.AddOfflineURLs(ctx, destinationID, urls)
}
