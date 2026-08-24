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
	if item.ID != target.ItemID || item.Name == "" || !isPlayableItemType(item.Type) {
		return playback.SourceMedia{}, playback.ErrNotFound
	}
	sources := cloudSourcesForItem(item)
	if len(sources) == 0 {
		return playback.SourceMedia{}, playback.ErrNotFound
	}
	playSessionID, err := randomPlaybackID()
	if err != nil {
		return playback.SourceMedia{}, playback.ErrUnavailable
	}
	var lastResolveErr error
	for _, source := range sources {
		session, err := c.playbackSession(target.ItemID, source.ID, playSessionID)
		if err != nil {
			return playback.SourceMedia{}, playback.ErrUnavailable
		}
		resolved, err := c.resolveCloudSource(ctx, configuration, target.ItemID, item, source, playSessionID, playbackUserAgent)
		if resolved.pickCode != "" || resolved.url != "" {
			return playback.SourceMedia{
				Name: item.Name, PickCode: resolved.pickCode, URL: resolved.url, Session: session,
				StartPositionMS: max(0, item.UserData.PlaybackPositionTicks/10_000),
			}, nil
		}
		if errors.Is(err, playback.ErrSourceUnauthorized) {
			return playback.SourceMedia{}, err
		}
		if err != nil && !errors.Is(err, errNoExternalPlayback) {
			lastResolveErr = err
		}
	}
	if lastResolveErr != nil {
		return playback.SourceMedia{}, fmt.Errorf("%w: %v", playback.ErrUnavailable, lastResolveErr)
	}
	return playback.SourceMedia{}, playback.ErrUnavailable
}

func cloudSourcesForItem(item baseItem) []mediaSource {
	sources := make([]mediaSource, 0, len(item.MediaSources))
	for _, source := range item.MediaSources {
		if validEmbyIdentifier(source.ID) && (is115Source(source) || is115Item(item)) {
			sources = append(sources, source)
		}
	}
	if len(sources) > 0 {
		return sources
	}
	if is115Item(item) && validEmbyIdentifier(item.ID) {
		return []mediaSource{{ID: item.ID, Path: item.Path, Container: "strm"}}
	}
	return nil
}

func isPlayableItemType(itemType string) bool {
	switch strings.TrimSpace(itemType) {
	case "Movie", "Episode", "Video":
		return true
	default:
		return false
	}
}

type cloudPlayback struct {
	pickCode string
	url      string
}

func (c *Client) resolveCloudSource(
	ctx context.Context,
	configuration clientConfig,
	itemID string,
	item baseItem,
	source mediaSource,
	playSessionID string,
	playbackUserAgent string,
) (cloudPlayback, error) {
	for _, value := range []string{source.DirectStreamURL, source.Path, item.Path} {
		if resolved := cloudPlaybackFromValue(value); resolved.pickCode != "" || resolved.url != "" {
			return resolved, nil
		}
	}
	if body, err := c.readStrmBody(ctx, configuration, itemID); err == nil {
		if resolved := cloudPlaybackFromValue(body); resolved.pickCode != "" || resolved.url != "" {
			return resolved, nil
		}
	}
	redirect, err := c.resolveExternalStreamRedirect(ctx, configuration, itemID, source, playSessionID, playbackUserAgent)
	if resolved := cloudPlaybackFromValue(redirect); resolved.pickCode != "" || resolved.url != "" {
		return resolved, nil
	}
	if err != nil {
		return cloudPlayback{}, err
	}
	return cloudPlayback{}, errNoExternalPlayback
}

func cloudPlaybackFromValue(value string) cloudPlayback {
	return cloudPlayback{pickCode: pickCodeFromValue(value), url: playbackURLFromValue(value)}
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
	playbackBaseURL := configuration.playbackBaseURL
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
	location, err := readRedirectLocation(c.client, request)
	if err != nil {
		return "", err
	}
	if cloudPlaybackFromValue(location).pickCode != "" || cloudPlaybackFromValue(location).url != "" {
		return location, nil
	}
	if hop, hopErr := c.followUnsignedRedirect(ctx, location, playbackUserAgent); hopErr == nil {
		return hop, nil
	}
	if !safeExternalPlaybackURL(location, configuration.baseURL, playbackBaseURL) {
		return location, errNoExternalPlayback
	}
	return location, nil
}

func (c *Client) followUnsignedRedirect(ctx context.Context, rawURL, playbackUserAgent string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errNoExternalPlayback
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "*/*")
	request.Header.Set("User-Agent", playbackUserAgent)
	return readRedirectLocation(c.client, request)
}

func readRedirectLocation(client *http.Client, request *http.Request) (string, error) {
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
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
