package emby

import (
	"bytes"
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
	"time"
)

const defaultSubtitleTimeout = 28 * time.Second

const (
	yearAsSeasonMin = 1900
	yearAsSeasonMax = 2100
)

var (
	seasonEpisodePattern = regexp.MustCompile(`(?i)(?:^|[^A-Za-z0-9])S(\d{1,2})E(\d{1,3})(?:[^A-Za-z0-9]|$)`)
	episodeOnlyPattern   = regexp.MustCompile(`(?i)(?:^|[^A-Za-z0-9])E(\d{1,3})(?:[^A-Za-z0-9]|$)`)
)

// RemoteSubtitle is a provider hit from Emby's remote subtitle search.
type RemoteSubtitle struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Language        string  `json:"language,omitempty"`
	Format          string  `json:"format,omitempty"`
	ProviderName    string  `json:"providerName,omitempty"`
	Author          string  `json:"author,omitempty"`
	Comment         string  `json:"comment,omitempty"`
	CommunityRating float64 `json:"communityRating,omitempty"`
	DownloadCount   int     `json:"downloadCount,omitempty"`
	IsHashMatch     bool    `json:"isHashMatch,omitempty"`
	HearingImpaired bool    `json:"hearingImpaired,omitempty"`
	Forced          bool    `json:"forced,omitempty"`
}

type remoteSubtitleInfo struct {
	ID                         string  `json:"Id"`
	Name                       string  `json:"Name"`
	ThreeLetterISOLanguageName string  `json:"ThreeLetterISOLanguageName"`
	Format                     string  `json:"Format"`
	ProviderName               string  `json:"ProviderName"`
	Author                     string  `json:"Author"`
	Comment                    string  `json:"Comment"`
	CommunityRating            float64 `json:"CommunityRating"`
	DownloadCount              int     `json:"DownloadCount"`
	IsHashMatch                bool    `json:"IsHashMatch"`
	HearingImpaired            bool    `json:"HearingImpaired"`
	Forced                     bool    `json:"Forced"`
}

// SearchRemoteSubtitles asks Emby subtitle providers for remote matches.
// Explicit "chi" searches Chinese only; empty/"zh" may also try "zh".
func (c *Client) SearchRemoteSubtitles(ctx context.Context, itemID, language string) ([]RemoteSubtitle, error) {
	configuration := c.configuration()
	if err := validateAuthenticated(configuration); err != nil {
		return nil, err
	}
	itemID = strings.TrimSpace(itemID)
	if itemID == "" || !validEmbyIdentifier(itemID) {
		return nil, ErrUpstreamResponse
	}
	c.prepareSubtitleSearchItem(ctx, configuration, itemID)
	languages := subtitleLanguageCandidates(language)
	seen := make(map[string]struct{})
	results := make([]RemoteSubtitle, 0, 16)
	for _, lang := range languages {
		endpoint, err := remoteSubtitleEndpoint(configuration.baseURL, itemID, lang)
		if err != nil {
			return nil, err
		}
		var payload []remoteSubtitleInfo
		if err := c.doJSONUsing(ctx, configuration, http.MethodGet, endpoint, nil, true, &payload, c.subtitleHTTP()); err != nil {
			return nil, err
		}
		for _, item := range payload {
			subtitle := publicRemoteSubtitle(item)
			if subtitle.ID == "" {
				continue
			}
			if _, exists := seen[subtitle.ID]; exists {
				continue
			}
			seen[subtitle.ID] = struct{}{}
			results = append(results, subtitle)
			if len(results) >= 40 {
				return results, nil
			}
		}
		if len(results) > 0 {
			break
		}
	}
	return results, nil
}

// DownloadRemoteSubtitle downloads one remote subtitle onto the Emby item media folder.
func (c *Client) DownloadRemoteSubtitle(ctx context.Context, itemID, subtitleID string) error {
	configuration := c.configuration()
	if err := validateAuthenticated(configuration); err != nil {
		return err
	}
	itemID = strings.TrimSpace(itemID)
	subtitleID = strings.TrimSpace(subtitleID)
	if itemID == "" || !validEmbyIdentifier(itemID) {
		return ErrUpstreamResponse
	}
	if err := validateSubtitleID(subtitleID); err != nil {
		return err
	}
	endpoint, err := remoteSubtitleEndpoint(configuration.baseURL, itemID, subtitleID)
	if err != nil {
		return err
	}
	if err := c.doJSONUsing(ctx, configuration, http.MethodPost, endpoint, map[string]any{}, true, nil, c.subtitleHTTP()); err != nil {
		return err
	}
	// Emby queues a refresh after download; ask again so UI can pick up streams sooner.
	_ = c.RefreshItem(ctx, itemID)
	return nil
}

func remoteSubtitleEndpoint(baseURL, itemID, finalSegment string) (string, error) {
	base, err := endpointURL(baseURL, path.Join("Items", itemID, "RemoteSearch", "Subtitles"), nil)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(base, "/") + "/" + url.PathEscape(finalSegment), nil
}

func subtitleLanguageCandidates(language string) []string {
	normalized := strings.ToLower(strings.TrimSpace(language))
	switch normalized {
	case "chi", "zho":
		return []string{"chi"}
	case "", "zh", "zh-cn", "zh-hans", "zh-sg", "chinese":
		return []string{"chi", "zh"}
	case "zh-tw", "zh-hk", "zh-hant":
		return []string{"zh-TW", "chi"}
	default:
		if len(normalized) < 2 || len(normalized) > 16 {
			return []string{"chi"}
		}
		return []string{normalized}
	}
}

func (c *Client) subtitleHTTP() *http.Client {
	if c.subtitleClient != nil {
		return c.subtitleClient
	}
	return c.client
}

func (c *Client) prepareSubtitleSearchItem(ctx context.Context, configuration clientConfig, itemID string) {
	raw, err := c.readItemObject(ctx, configuration, itemID)
	if err != nil {
		return
	}
	itemType, _ := raw["Type"].(string)
	season := jsonNumber(raw["ParentIndexNumber"])
	episode := jsonNumber(raw["IndexNumber"])
	nextSeason, nextEpisode, changed := normalizeEpisodeNumbering(itemType, season, episode, itemMediaPath(raw))
	if !changed {
		return
	}
	raw["ParentIndexNumber"] = nextSeason
	raw["IndexNumber"] = nextEpisode
	_ = c.postJSONBody(ctx, configuration, path.Join("Items", itemID), nil, raw, nil, false)
}

func (c *Client) readItemObject(ctx context.Context, configuration clientConfig, itemID string) (map[string]any, error) {
	query := url.Values{"Fields": {"MediaSources,Path,ProviderIds"}}
	endpointPath := path.Join("Items", itemID)
	if configuration.userID != "" {
		endpointPath = path.Join("Users", configuration.userID, "Items", itemID)
	}
	var raw map[string]any
	if err := c.getJSONWithNotFound(ctx, configuration, endpointPath, query, true, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func normalizeEpisodeNumbering(itemType string, season, episode int, mediaPath string) (int, int, bool) {
	if !strings.EqualFold(strings.TrimSpace(itemType), "Episode") {
		return season, episode, false
	}
	parsedSeason, parsedEpisode, parsed := parseSeasonEpisode(mediaPath)
	nextSeason, nextEpisode := season, episode
	if yearLikeSeason(season) {
		if parsed && parsedSeason > 0 && !yearLikeSeason(parsedSeason) {
			nextSeason = parsedSeason
		} else {
			nextSeason = 1
		}
	}
	if nextEpisode <= 0 && parsed && parsedEpisode > 0 {
		nextEpisode = parsedEpisode
	}
	return nextSeason, nextEpisode, nextSeason != season || nextEpisode != episode
}

func yearLikeSeason(season int) bool {
	return season >= yearAsSeasonMin && season <= yearAsSeasonMax
}

func parseSeasonEpisode(mediaPath string) (int, int, bool) {
	base := path.Base(strings.ReplaceAll(mediaPath, "\\", "/"))
	if match := seasonEpisodePattern.FindStringSubmatch(base); len(match) == 3 {
		season, seasonErr := strconv.Atoi(match[1])
		episode, episodeErr := strconv.Atoi(match[2])
		if seasonErr == nil && episodeErr == nil && season >= 0 && episode > 0 {
			return season, episode, true
		}
	}
	if match := episodeOnlyPattern.FindStringSubmatch(base); len(match) == 2 {
		episode, err := strconv.Atoi(match[1])
		if err == nil && episode > 0 {
			return 0, episode, true
		}
	}
	return 0, 0, false
}

func itemMediaPath(raw map[string]any) string {
	if value, ok := raw["Path"].(string); ok && strings.TrimSpace(value) != "" {
		return value
	}
	sources, _ := raw["MediaSources"].([]any)
	for _, source := range sources {
		object, _ := source.(map[string]any)
		if object == nil {
			continue
		}
		if value, ok := object["Path"].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func jsonNumber(value any) int {
	switch number := value.(type) {
	case int:
		return number
	case int64:
		return int(number)
	case float64:
		return int(number)
	case json.Number:
		parsed, err := number.Int64()
		if err != nil {
			return 0
		}
		return int(parsed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(number))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func publicRemoteSubtitle(item remoteSubtitleInfo) RemoteSubtitle {
	return RemoteSubtitle{
		ID:              strings.TrimSpace(item.ID),
		Name:            boundedText(item.Name, 240),
		Language:        boundedText(item.ThreeLetterISOLanguageName, 16),
		Format:          boundedText(item.Format, 32),
		ProviderName:    boundedText(item.ProviderName, 80),
		Author:          boundedText(item.Author, 80),
		Comment:         boundedText(item.Comment, 240),
		CommunityRating: item.CommunityRating,
		DownloadCount:   max(0, item.DownloadCount),
		IsHashMatch:     item.IsHashMatch,
		HearingImpaired: item.HearingImpaired,
		Forced:          item.Forced,
	}
}

func validateSubtitleID(subtitleID string) error {
	if subtitleID == "" || len(subtitleID) > 512 {
		return fmt.Errorf("%w: subtitle id is invalid", ErrUpstreamResponse)
	}
	for _, character := range subtitleID {
		if character < 0x20 || character == 0x7f {
			return fmt.Errorf("%w: subtitle id is invalid", ErrUpstreamResponse)
		}
	}
	return nil
}

func (c *Client) doJSON(
	ctx context.Context,
	configuration clientConfig,
	method string,
	endpoint string,
	body any,
	mapNotFound bool,
	target any,
) error {
	return c.doJSONUsing(ctx, configuration, method, endpoint, body, mapNotFound, target, c.client)
}

func (c *Client) doJSONUsing(
	ctx context.Context,
	configuration clientConfig,
	method string,
	endpoint string,
	body any,
	mapNotFound bool,
	target any,
	httpClient *http.Client,
) error {
	if httpClient == nil {
		httpClient = c.client
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode Emby request: %w", err)
		}
		reader = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("create Emby request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Media-Hub/emby")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	applyEmbyAuth(request, configuration)

	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("request Emby: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	if mapNotFound && response.StatusCode == http.StatusNotFound {
		return ErrItemNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ErrUpstreamResponse
	}
	return decodeEmbyBody(response, target)
}
