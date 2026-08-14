package drive115

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
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
	cookie := c.session()
	if cookie == "" {
		return ErrNotConfigured
	}
	userID, err := c.shareUserID(ctx, cookie)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		selected, err = c.shareRootItemIDs(ctx, cookie, shareCode, receiveCode)
		if err != nil {
			return err
		}
	}

	values := url.Values{
		"user_id":      {userID},
		"share_code":   {shareCode},
		"receive_code": {receiveCode},
		"file_id":      {strings.Join(selected, ",")},
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

const (
	shareSnapPageSize = 100
	shareSnapMaxItems = 1000
)

func normalizeShareFileIDs(fileIDs []string) ([]string, error) {
	if len(fileIDs) > shareSnapMaxItems {
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

func (c *Client) shareUserID(ctx context.Context, cookie string) (string, error) {
	var payload struct {
		State bool `json:"state"`
		Data  struct {
			UID json.RawMessage `json:"uid"`
		} `json:"data"`
	}
	if err := c.getJSONWithSession(ctx, c.shareUserURL, nil, cookie, &payload); err != nil {
		return "", err
	}
	userID := shareID(payload.Data.UID)
	if !payload.State || userID == "" {
		return "", ErrUnauthorized
	}
	return userID, nil
}

func (c *Client) shareRootItemIDs(ctx context.Context, cookie, shareCode, receiveCode string) ([]string, error) {
	type shareItem struct {
		FileID json.RawMessage `json:"fid"`
		DirID  json.RawMessage `json:"cid"`
		IsFile json.RawMessage `json:"fc"`
	}
	seen := make(map[string]struct{})
	selected := make([]string, 0)
	expected := -1
	offset := 0
	for {
		query := url.Values{
			"share_code":   {shareCode},
			"receive_code": {receiveCode},
			"cid":          {""},
			"offset":       {strconv.Itoa(offset)},
			"limit":        {strconv.Itoa(shareSnapPageSize)},
		}
		var payload struct {
			State bool `json:"state"`
			Data  struct {
				Count json.RawMessage `json:"count"`
				List  []shareItem     `json:"list"`
			} `json:"data"`
		}
		if err := c.getJSONWithSession(ctx, c.shareSnapURL, query, cookie, &payload); err != nil {
			return nil, err
		}
		count, ok := shareInteger(payload.Data.Count)
		if !payload.State || !ok || count <= 0 || count > shareSnapMaxItems {
			return nil, ErrUpstreamResponse
		}
		if expected < 0 {
			expected = count
		} else if count != expected {
			return nil, ErrUpstreamResponse
		}
		if len(payload.Data.List) == 0 || offset+len(payload.Data.List) > expected {
			return nil, ErrUpstreamResponse
		}
		for _, item := range payload.Data.List {
			isFile, ok := shareInteger(item.IsFile)
			if !ok || (isFile != 0 && isFile != 1) {
				return nil, ErrUpstreamResponse
			}
			id := shareID(item.DirID)
			if isFile == 1 {
				id = shareID(item.FileID)
			}
			if id == "" {
				return nil, ErrUpstreamResponse
			}
			if _, exists := seen[id]; exists {
				return nil, ErrUpstreamResponse
			}
			seen[id] = struct{}{}
			selected = append(selected, id)
		}
		offset += len(payload.Data.List)
		if offset == expected {
			break
		}
	}
	if len(selected) != expected {
		return nil, ErrUpstreamResponse
	}
	return selected, nil
}

func shareID(raw json.RawMessage) string {
	text := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if !numericIDPattern.MatchString(text) || text == "0" {
		return ""
	}
	return text
}

func shareInteger(raw json.RawMessage) (int, bool) {
	text := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if text == "" || len(text) > 10 {
		return 0, false
	}
	value, err := strconv.Atoi(text)
	if err != nil || value < 0 {
		return 0, false
	}
	return value, true
}

func (s *AuthService) ReceiveShare(ctx context.Context, destinationID, shareCode, receiveCode string, fileIDs []string) error {
	if err := s.prepareSession(ctx); err != nil {
		return err
	}
	return s.drive.ReceiveShare(ctx, destinationID, shareCode, receiveCode, fileIDs)
}
