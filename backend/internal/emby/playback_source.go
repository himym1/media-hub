package emby

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"media-hub/backend/internal/playback"
)

var errNoExternalPlayback = errors.New("Emby media source has no external playback redirect")

type playbackReference struct {
	ItemID        string `json:"itemId"`
	MediaSourceID string `json:"mediaSourceId"`
	PlaySessionID string `json:"playSessionId"`
}

func (c *Client) ResolveEmbyItem(ctx context.Context, target playback.EmbyItemTarget, playbackUserAgent string) (playback.SourceMedia, error) {
	configuration := c.configuration()
	if err := validateAuthenticated(configuration); err != nil {
		return playback.SourceMedia{}, normalizePlaybackError(err)
	}
	if !validEmbyIdentifier(target.ItemID) || strings.TrimSpace(playbackUserAgent) == "" {
		return playback.SourceMedia{}, playback.ErrInvalidRequest
	}
	itemPath := path.Join("Items", target.ItemID)
	if configuration.userID != "" {
		itemPath = path.Join("Users", configuration.userID, "Items", target.ItemID)
	}
	var item baseItem
	if err := c.getJSONWithNotFound(ctx, configuration, itemPath, url.Values{"Fields": {"MediaSources,UserData"}}, true, &item); err != nil {
		return playback.SourceMedia{}, normalizePlaybackError(err)
	}
	if item.ID != target.ItemID || item.Name == "" || item.Type == "Series" {
		return playback.SourceMedia{}, playback.ErrNotFound
	}
	query := url.Values{}
	if configuration.userID != "" {
		query.Set("UserId", configuration.userID)
	}
	var response playbackInfoResponse
	if err := c.postJSON(ctx, configuration, path.Join("Items", target.ItemID, "PlaybackInfo"), query, &response); err != nil {
		return playback.SourceMedia{}, normalizePlaybackError(err)
	}
	playSessionID := strings.TrimSpace(response.PlaySessionID)
	if playSessionID == "" {
		var err error
		playSessionID, err = randomPlaybackID()
		if err != nil {
			return playback.SourceMedia{}, playback.ErrUnavailable
		}
	}
	hasCloudSource := false
	for _, source := range response.MediaSources {
		if !validEmbyIdentifier(source.ID) || (!is115Source(source) && !is115Path(item.Path)) {
			continue
		}
		hasCloudSource = true
		session, err := c.playbackSession(target.ItemID, source.ID, playSessionID)
		if err != nil {
			return playback.SourceMedia{}, playback.ErrUnavailable
		}
		media := playback.SourceMedia{
			Name: item.Name, Session: session,
			StartPositionMS: max(0, item.UserData.PlaybackPositionTicks/10_000),
		}
		if code := firstNonEmpty(pickCodeFromValue(source.DirectStreamURL), pickCodeFromValue(source.Path)); code != "" {
			media.PickCode = code
			return media, nil
		}
		for _, candidate := range []string{source.DirectStreamURL, source.Path} {
			if safeExternalPlaybackURL(candidate, configuration.baseURL, c.playbackBaseURL) {
				media.URL = strings.TrimSpace(candidate)
				return media, nil
			}
		}
		if body, readErr := c.readStrmBody(ctx, configuration, target.ItemID); readErr == nil {
			if code := pickCodeFromValue(body); code != "" {
				media.PickCode = code
				return media, nil
			}
			if line := firstNonEmpty(strings.Split(body, "\n")...); safeExternalPlaybackURL(line, configuration.baseURL, c.playbackBaseURL) {
				media.URL = strings.TrimSpace(line)
				return media, nil
			}
		}
		redirect, err := c.resolveExternalStreamRedirect(
			ctx, configuration, target.ItemID, source, playSessionID, playbackUserAgent,
		)
		if code := pickCodeFromValue(redirect); code != "" {
			media.PickCode = code
			return media, nil
		}
		if err == nil && redirect != "" {
			media.URL = redirect
			return media, nil
		}
		if errors.Is(err, errNoExternalPlayback) {
			continue
		}
		if errors.Is(err, playback.ErrSourceUnauthorized) {
			return playback.SourceMedia{}, err
		}
		return playback.SourceMedia{}, fmt.Errorf("%w: Emby playback facade request failed", playback.ErrUnavailable)
	}
	if !hasCloudSource {
		return playback.SourceMedia{}, playback.ErrNotFound
	}
	return playback.SourceMedia{}, playback.ErrUnavailable
}

func (c *Client) playbackSession(itemID, mediaSourceID, playSessionID string) (*playback.SourceSession, error) {
	value, err := json.Marshal(playbackReference{
		ItemID: itemID, MediaSourceID: mediaSourceID, PlaySessionID: playSessionID,
	})
	if err != nil {
		return nil, err
	}
	return &playback.SourceSession{Reporter: c, Reference: string(value)}, nil
}

func (c *Client) ReportPlayback(ctx context.Context, reference string, event playback.SessionEvent) error {
	configuration := c.configuration()
	if err := validateAuthenticated(configuration); err != nil {
		return normalizePlaybackError(err)
	}
	var target playbackReference
	if err := json.Unmarshal([]byte(reference), &target); err != nil ||
		!validEmbyIdentifier(target.ItemID) || !validEmbyIdentifier(target.MediaSourceID) || target.PlaySessionID == "" {
		return playback.ErrInvalidRequest
	}
	endpointPath := "Sessions/Playing/Progress"
	switch event.Type {
	case playback.SessionStarted:
		endpointPath = "Sessions/Playing"
	case playback.SessionStopped:
		endpointPath = "Sessions/Playing/Stopped"
	case playback.SessionProgress:
	default:
		return playback.ErrInvalidRequest
	}
	payload := map[string]any{
		"ItemId":        target.ItemID,
		"MediaSourceId": target.MediaSourceID,
		"PlaySessionId": target.PlaySessionID,
		"PositionTicks": event.PositionMS * 10_000,
		"IsPaused":      event.Paused,
		"CanSeek":       true,
		"PlayMethod":    "DirectPlay",
	}
	return c.postPlaybackEvent(ctx, configuration, endpointPath, payload)
}

func (c *Client) postPlaybackEvent(ctx context.Context, configuration clientConfig, endpointPath string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint, err := endpointURL(configuration.baseURL, endpointPath, nil)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Media-Hub/emby-playback")
	request.Header.Set("X-Emby-Token", configuration.apiKey)
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return playback.ErrSourceUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return playback.ErrUnavailable
	}
	return nil
}

func (c *Client) resolveExternalStreamRedirect(
	ctx context.Context,
	configuration clientConfig,
	itemID string,
	source mediaSource,
	playSessionID string,
	playbackUserAgent string,
) (string, error) {
	extension := strings.Trim(strings.ToLower(source.Container), ".")
	endpointPath := path.Join("Videos", itemID, "stream")
	if extension != "" && len(extension) <= 16 {
		endpointPath += "." + extension
	}
	query := url.Values{
		"MediaSourceId":     {source.ID},
		"PlaySessionId":     {playSessionID},
		"Static":            {"true"},
		"EnableRedirection": {"true"},
		"EnableRemoteMedia": {"true"},
	}
	playbackBaseURL := c.playbackBaseURL
	if playbackBaseURL == "" {
		playbackBaseURL = configuration.baseURL
	}
	endpoint, err := endpointURL(playbackBaseURL, endpointPath, query)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "*/*")
	// Range requests make some Emby versions proxy remote STRM content instead of returning the configured redirect.
	request.Header.Set("User-Agent", playbackUserAgent)
	request.Header.Set("X-Emby-Token", configuration.apiKey)
	response, err := c.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return "", playback.ErrSourceUnauthorized
	}
	if response.StatusCode < 300 || response.StatusCode > 399 {
		return "", errNoExternalPlayback
	}
	location, err := response.Location()
	if err != nil {
		return "", errNoExternalPlayback
	}
	if !safeExternalPlaybackURL(location.String(), configuration.baseURL, playbackBaseURL) {
		return location.String(), errNoExternalPlayback
	}
	return location.String(), nil
}

func safeExternalPlaybackURL(value string, blockedBaseURLs ...string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return false
	}
	for _, blocked := range blockedBaseURLs {
		blockedURL, blockedErr := url.Parse(strings.TrimSpace(blocked))
		if blockedErr == nil && strings.EqualFold(parsed.Host, blockedURL.Host) {
			return false
		}
	}
	return true
}

func randomPlaybackID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func normalizePlaybackError(err error) error {
	switch {
	case errors.Is(err, ErrNotConfigured), errors.Is(err, ErrMissingAPIKey):
		return fmt.Errorf("%w: %v", playback.ErrSourceNotConfigured, err)
	case errors.Is(err, ErrUnauthorized):
		return fmt.Errorf("%w: %v", playback.ErrSourceUnauthorized, err)
	case errors.Is(err, ErrItemNotFound):
		return fmt.Errorf("%w: %v", playback.ErrNotFound, err)
	default:
		return err
	}
}

func (c *Client) readStrmBody(ctx context.Context, configuration clientConfig, itemID string) (string, error) {
	endpoint, err := endpointURL(configuration.baseURL, path.Join("Items", itemID, "Download"), nil)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "text/plain, application/octet-stream")
	request.Header.Set("User-Agent", "Media-Hub/emby-playback")
	request.Header.Set("X-Emby-Token", configuration.apiKey)
	response, err := c.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", errNoExternalPlayback
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4096))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

var (
	_ playback.EmbyResolver    = (*Client)(nil)
	_ playback.SessionReporter = (*Client)(nil)
)
