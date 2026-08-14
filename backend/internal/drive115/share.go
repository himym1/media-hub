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

var (
	shareCodePattern   = regexp.MustCompile(`^[A-Za-z0-9]{4,64}$`)
	receiveCodePattern = regexp.MustCompile(`^[A-Za-z0-9]{0,8}$`)
	numericIDPattern   = regexp.MustCompile(`^[0-9]{1,32}$`)
)

func (c *Client) ReceiveShare(ctx context.Context, destinationID, shareCode, receiveCode string, fileIDs []string) error {
	shareCode = strings.TrimSpace(shareCode)
	receiveCode = strings.TrimSpace(receiveCode)
	destinationID = strings.TrimSpace(destinationID)
	if destinationID == "" {
		destinationID = "0"
	}
	if !shareCodePattern.MatchString(shareCode) || !receiveCodePattern.MatchString(receiveCode) || !numericIDPattern.MatchString(destinationID) {
		return &WriteError{Code: "invalid_request", Err: ErrUpstreamResponse}
	}

	selected, err := normalizeShareFileIDs(fileIDs)
	if err != nil {
		return err
	}
	fileID := "0"
	if len(selected) > 0 {
		fileID = strings.Join(selected, ",")
	}
	cookie := c.session()
	if cookie == "" {
		return ErrNotConfigured
	}
	userID := shareUserID(cookie)
	if userID == "" {
		return ErrUnauthorized
	}

	values := url.Values{
		"user_id":      {userID},
		"share_code":   {shareCode},
		"receive_code": {receiveCode},
		"file_id":      {fileID},
		"cid":          {destinationID},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.shareReceiveURL, strings.NewReader(values.Encode()))
	if err != nil {
		return &WriteError{Code: "invalid_request", Err: err}
	}
	c.applyAuth(request, cookie)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.client.Do(request)
	if err != nil {
		return &WriteError{Uncertain: true, Code: "uncertain_result", Err: fmt.Errorf("receive 115 share: %w", err)}
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &WriteError{Code: "provider_rejected", Err: ErrUpstreamResponse}
	}
	var payload struct {
		State    bool            `json:"state"`
		Errno    json.RawMessage `json:"errno"`
		Error    string          `json:"error"`
		ErrorMsg string          `json:"error_msg"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return &WriteError{Uncertain: true, Code: "invalid_response", Err: err}
	}
	errno := strings.Trim(string(payload.Errno), `"`)
	if payload.State || errno == "4100024" {
		return nil
	}
	return &WriteError{Code: "provider_rejected", Err: ErrUpstreamResponse}
}

const shareMaxSelectedItems = 1000

func normalizeShareFileIDs(fileIDs []string) ([]string, error) {
	if len(fileIDs) > shareMaxSelectedItems {
		return nil, &WriteError{Code: "invalid_request", Err: ErrUpstreamResponse}
	}
	seen := make(map[string]struct{}, len(fileIDs))
	selected := make([]string, 0, len(fileIDs))
	for _, id := range fileIDs {
		id = strings.TrimSpace(id)
		if !numericIDPattern.MatchString(id) || id == "0" {
			return nil, &WriteError{Code: "invalid_request", Err: ErrUpstreamResponse}
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		selected = append(selected, id)
	}
	return selected, nil
}

func shareUserID(cookieHeader string) string {
	request := http.Request{Header: make(http.Header)}
	request.Header.Set("Cookie", cookieHeader)
	cookie, err := request.Cookie("UID")
	if err != nil {
		return ""
	}
	userID, _, _ := strings.Cut(strings.TrimSpace(cookie.Value), "_")
	if !numericIDPattern.MatchString(userID) || userID == "0" {
		return ""
	}
	return userID
}

func (s *AuthService) ReceiveShare(ctx context.Context, destinationID, shareCode, receiveCode string, fileIDs []string) error {
	if err := s.prepareSession(ctx); err != nil {
		return err
	}
	return s.drive.ReceiveShare(ctx, destinationID, shareCode, receiveCode, fileIDs)
}
