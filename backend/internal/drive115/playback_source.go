package drive115

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"media-hub/backend/internal/playback"
)

type playbackFileInfo struct {
	FileID   string
	ParentID string
	Name     string
	PickCode string
}

func (c *Client) resolvePlayback(ctx context.Context, target playback.Drive115Target, playbackUserAgent string) (playback.SourceMedia, error) {
	if !numericIDPattern.MatchString(target.ParentID) || !numericIDPattern.MatchString(target.FileID) || strings.TrimSpace(playbackUserAgent) == "" {
		return playback.SourceMedia{}, playback.ErrInvalidRequest
	}
	info, err := c.playbackFileInfo(ctx, target.FileID)
	if err != nil {
		return playback.SourceMedia{}, normalizePlaybackSourceError(err)
	}
	if info.FileID != target.FileID || info.ParentID != target.ParentID || info.Name == "" || info.PickCode == "" {
		return playback.SourceMedia{}, playback.ErrNotFound
	}
	streamURL, err := c.resolveDownloadURL(ctx, info.PickCode, playbackUserAgent)
	if err != nil {
		return playback.SourceMedia{}, normalizePlaybackSourceError(err)
	}
	return playback.SourceMedia{URL: streamURL, Name: info.Name}, nil
}

func (c *Client) playbackFileInfo(ctx context.Context, fileID string) (playbackFileInfo, error) {
	cookie := c.session()
	if cookie == "" {
		return playbackFileInfo{}, ErrNotConfigured
	}
	var payload struct {
		State bool `json:"state"`
		Data  []struct {
			FileID      string `json:"fid"`
			AltFileID   string `json:"file_id"`
			ParentID    string `json:"cid"`
			AltParent   string `json:"pid"`
			CategoryID  string `json:"category_id"`
			Name        string `json:"n"`
			AltName     string `json:"file_name"`
			PickCode    string `json:"pc"`
			AltPickCode string `json:"pick_code"`
		} `json:"data"`
	}
	query := url.Values{"file_id": {fileID}}
	if err := c.getJSONWithSession(ctx, c.fileInfoURL, query, cookie, &payload); err != nil {
		return playbackFileInfo{}, err
	}
	if !payload.State || len(payload.Data) != 1 {
		return playbackFileInfo{}, playback.ErrNotFound
	}
	item := payload.Data[0]
	return playbackFileInfo{
		FileID:   firstNonBlank(item.FileID, item.AltFileID),
		ParentID: firstNonBlank(item.ParentID, item.AltParent, item.CategoryID),
		Name:     firstNonBlank(item.Name, item.AltName),
		PickCode: firstNonBlank(item.PickCode, item.AltPickCode),
	}, nil
}

func (c *Client) resolveDownloadURL(ctx context.Context, pickCode, playbackUserAgent string) (string, error) {
	cookie := c.session()
	if cookie == "" {
		return "", ErrNotConfigured
	}
	endpoint, err := url.Parse(c.downloadURL)
	if err != nil {
		return "", ErrUpstreamResponse
	}
	query := endpoint.Query()
	query.Set("pickcode", pickCode)
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", ErrUpstreamResponse
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Cookie", cookie)
	request.Header.Set("User-Agent", playbackUserAgent)
	response, err := c.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return "", ErrUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", ErrUpstreamResponse
	}
	var payload struct {
		State   bool            `json:"state"`
		FileURL string          `json:"file_url"`
		URL     json.RawMessage `json:"url"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil || !payload.State {
		return "", ErrUpstreamResponse
	}
	if value := strings.TrimSpace(payload.FileURL); value != "" {
		return value, nil
	}
	var direct string
	if err := json.Unmarshal(payload.URL, &direct); err == nil && strings.TrimSpace(direct) != "" {
		return strings.TrimSpace(direct), nil
	}
	var nested struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(payload.URL, &nested); err == nil && strings.TrimSpace(nested.URL) != "" {
		return strings.TrimSpace(nested.URL), nil
	}
	return "", ErrUpstreamResponse
}

func normalizePlaybackSourceError(err error) error {
	switch {
	case errors.Is(err, ErrNotConfigured):
		return fmt.Errorf("%w: %v", playback.ErrSourceNotConfigured, err)
	case errors.Is(err, ErrUnauthorized):
		return fmt.Errorf("%w: %v", playback.ErrSourceUnauthorized, err)
	default:
		return err
	}
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
