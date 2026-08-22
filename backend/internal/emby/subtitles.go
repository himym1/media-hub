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
	"strings"
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
// language defaults to Chinese (chi); zh aliases also try chi when empty.
func (c *Client) SearchRemoteSubtitles(ctx context.Context, itemID, language string) ([]RemoteSubtitle, error) {
	configuration := c.configuration()
	if err := validateAuthenticated(configuration); err != nil {
		return nil, err
	}
	itemID = strings.TrimSpace(itemID)
	if itemID == "" || !validEmbyIdentifier(itemID) {
		return nil, ErrUpstreamResponse
	}
	languages := subtitleLanguageCandidates(language)
	seen := make(map[string]struct{})
	results := make([]RemoteSubtitle, 0, 16)
	for _, lang := range languages {
		endpoint, err := remoteSubtitleEndpoint(configuration.baseURL, itemID, lang)
		if err != nil {
			return nil, err
		}
		var payload []remoteSubtitleInfo
		if err := c.doJSON(ctx, configuration, http.MethodGet, endpoint, nil, true, &payload); err != nil {
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
	if err := c.doJSON(ctx, configuration, http.MethodPost, endpoint, map[string]any{}, true, nil); err != nil {
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
	case "", "zh", "zh-cn", "zh-hans", "zh-sg", "chinese", "chi", "zho":
		return []string{"chi", "zh"}
	case "zh-tw", "zh-hk", "zh-hant":
		return []string{"zh-TW", "chi", "zh"}
	default:
		if len(normalized) < 2 || len(normalized) > 16 {
			return []string{"chi", "zh"}
		}
		return []string{normalized}
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

	response, err := c.client.Do(request)
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
	if target == nil || response.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(target); err != nil {
		return fmt.Errorf("decode Emby response: %w", err)
	}
	return nil
}
