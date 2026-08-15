package drive115

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
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
	userID := shareUserID(cookie)
	if userID == "" {
		return ErrUnauthorized
	}
	if len(selected) == 0 {
		_, rootIDs, inspectErr := c.InspectShare(ctx, shareCode, receiveCode)
		if inspectErr != nil {
			return inspectErr
		}
		selected, err = normalizeShareFileIDs(rootIDs)
		if err != nil || len(selected) == 0 {
			return &WriteError{Code: "invalid_request", Err: ErrUpstreamResponse}
		}
	}
	fileID := strings.Join(selected, ",")

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
	if payload.State || errno == "4100024" || errno == "4000023" {
		return nil
	}
	return &WriteError{Code: "provider_rejected", Err: ErrUpstreamResponse}
}

const (
	shareMaxSelectedItems = 1000
	shareSnapPageSize     = 100
	shareSnapMaxEntries   = 1000
	shareSnapMaxDepth     = 8
)

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

type shareDirectory struct {
	id    string
	depth int
}

func (c *Client) InspectShare(ctx context.Context, shareCode, receiveCode string) ([]string, []string, error) {
	shareCode = strings.TrimSpace(shareCode)
	receiveCode = strings.TrimSpace(receiveCode)
	if !shareCodePattern.MatchString(shareCode) || !receiveCodePattern.MatchString(receiveCode) {
		return nil, nil, ErrUpstreamResponse
	}
	cookie := c.session()
	if cookie == "" {
		return nil, nil, ErrNotConfigured
	}

	type shareItem struct {
		FileID json.RawMessage `json:"fid"`
		DirID  json.RawMessage `json:"cid"`
		IsFile json.RawMessage `json:"fc"`
		Name   string          `json:"n"`
	}
	directories := []shareDirectory{{}}
	visited := map[string]struct{}{"__root__": {}}
	names := make([]string, 0)
	rootIDs := make([]string, 0)
	totalEntries := 0
	for len(directories) > 0 {
		current := directories[0]
		directories = directories[1:]
		offset := 0
		expected := -1
		directoryBaseEntries := totalEntries
		for {
			query := url.Values{
				"share_code":   {shareCode},
				"receive_code": {receiveCode},
				"cid":          {current.id},
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
				return nil, nil, err
			}
			if !payload.State {
				return nil, nil, ErrUpstreamResponse
			}
			if count, ok := shareInteger(payload.Data.Count); ok {
				if count > shareSnapMaxEntries-directoryBaseEntries {
					return nil, nil, ErrUpstreamResponse
				}
				if expected < 0 {
					expected = count
				} else if count != expected {
					return nil, nil, ErrUpstreamResponse
				}
			} else if len(payload.Data.Count) != 0 && string(payload.Data.Count) != "null" {
				return nil, nil, ErrUpstreamResponse
			}
			if len(payload.Data.List) == 0 {
				if expected >= 0 && offset != expected {
					return nil, nil, ErrUpstreamResponse
				}
				break
			}
			if expected >= 0 && offset+len(payload.Data.List) > expected {
				return nil, nil, ErrUpstreamResponse
			}
			for _, item := range payload.Data.List {
				totalEntries++
				if totalEntries > shareSnapMaxEntries {
					return nil, nil, ErrUpstreamResponse
				}
				isFile, ok := shareInteger(item.IsFile)
				name := strings.TrimSpace(item.Name)
				if !ok || (isFile != 0 && isFile != 1) || name == "" || len([]rune(name)) > 500 || strings.ContainsAny(name, "/\\\r\n\x00") {
					return nil, nil, ErrUpstreamResponse
				}
				if isFile == 1 {
					fileID := shareID(item.FileID)
					if fileID == "" {
						return nil, nil, ErrUpstreamResponse
					}
					if current.depth == 0 {
						rootIDs = append(rootIDs, fileID)
					}
					if shareVideoName(name) {
						names = append(names, name)
					}
					continue
				}
				directoryID := shareID(item.DirID)
				if directoryID == "" || current.depth >= shareSnapMaxDepth {
					return nil, nil, ErrUpstreamResponse
				}
				if current.depth == 0 {
					rootIDs = append(rootIDs, directoryID)
				}
				if _, exists := visited[directoryID]; exists {
					return nil, nil, ErrUpstreamResponse
				}
				visited[directoryID] = struct{}{}
				directories = append(directories, shareDirectory{id: directoryID, depth: current.depth + 1})
			}
			offset += len(payload.Data.List)
			if expected >= 0 {
				if offset == expected {
					break
				}
			} else if len(payload.Data.List) < shareSnapPageSize {
				break
			}
		}
	}
	if len(names) == 0 || len(rootIDs) == 0 {
		return nil, nil, ErrUpstreamResponse
	}
	return names, rootIDs, nil
}

func shareVideoName(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".3gp", ".avi", ".flv", ".iso", ".m2ts", ".m4v", ".mkv", ".mov", ".mp4", ".mpeg", ".mpg", ".ts", ".webm", ".wmv":
		return true
	default:
		return false
	}
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

func (s *AuthService) InspectShare(ctx context.Context, shareCode, receiveCode string) ([]string, []string, error) {
	if err := s.prepareSession(ctx); err != nil {
		return nil, nil, err
	}
	return s.drive.InspectShare(ctx, shareCode, receiveCode)
}
