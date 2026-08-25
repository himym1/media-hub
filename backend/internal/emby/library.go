package emby

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
)

type Episode struct {
	Item
	ExternalURL string `json:"externalUrl"`
	AppURL      string `json:"appUrl,omitempty"`
}

type PrimaryImage struct {
	Data        []byte
	ContentType string
}

func (c *Client) Episodes(ctx context.Context, seriesID string) ([]Episode, error) {
	configuration := c.configuration()
	if err := validateAuthenticated(configuration); err != nil {
		return nil, err
	}
	seriesID = strings.TrimSpace(seriesID)
	if !validEmbyIdentifier(seriesID) {
		return nil, ErrItemNotFound
	}
	query := url.Values{
		"Fields":    {"ProviderIds,MediaSources,UserData,Path,SeriesName"},
		"IsMissing": {"false"},
		"SortBy":    {"ParentIndexNumber,IndexNumber"},
		"SortOrder": {"Ascending"},
		"Limit":     {"10000"},
	}
	if configuration.userID != "" {
		query.Set("UserId", configuration.userID)
	}
	var response itemResponse
	if err := c.getJSON(ctx, configuration, path.Join("Shows", seriesID, "Episodes"), query, true, &response); err != nil {
		return nil, err
	}
	server, err := c.readServerInfo(ctx, configuration, "System/Info", true)
	if err != nil {
		return nil, err
	}
	items := make([]Episode, 0, len(response.Items))
	for _, item := range response.Items {
		if item.ID == "" || item.Type != "Episode" || !is115Item(item) {
			continue
		}
		item = presentEpisodeItem(item)
		externalURL, err := itemWebURL(configuration.baseURL, item.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, Episode{
			Item: publicItem(item), ExternalURL: externalURL, AppURL: itemAppURL(server.ID, item.ID),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Season != items[j].Season {
			return items[i].Season < items[j].Season
		}
		return items[i].Episode < items[j].Episode
	})
	return items, nil
}

func (c *Client) PrimaryImage(ctx context.Context, itemID string, maxWidth int) (PrimaryImage, error) {
	configuration := c.configuration()
	if err := validateAuthenticated(configuration); err != nil {
		return PrimaryImage{}, err
	}
	if !validEmbyIdentifier(itemID) {
		return PrimaryImage{}, ErrItemNotFound
	}
	if maxWidth < 64 || maxWidth > 640 {
		maxWidth = 320
	}
	endpoint, err := endpointURL(configuration.baseURL, path.Join("Items", itemID, "Images", "Primary"), url.Values{
		"maxWidth": {strconv.Itoa(maxWidth)},
		"quality":  {"85"},
	})
	if err != nil {
		return PrimaryImage{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return PrimaryImage{}, err
	}
	// Prefer JPEG: some Emby builds return 500 when asked to convert Primary to WebP.
	request.Header.Set("Accept", "image/jpeg,image/png,image/webp,*/*")
	request.Header.Set("User-Agent", "Media-Hub/emby-image")
	request.Header.Set("X-Emby-Token", configuration.apiKey)
	response, err := c.imageClient.Do(request)
	if err != nil {
		return PrimaryImage{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return PrimaryImage{}, ErrItemNotFound
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return PrimaryImage{}, ErrUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return PrimaryImage{}, ErrUpstreamResponse
	}
	contentType := strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0])
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" {
		return PrimaryImage{}, ErrUpstreamResponse
	}
	const limit = 2 << 20
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || len(data) == 0 || len(data) > limit {
		return PrimaryImage{}, fmt.Errorf("read Emby primary image: %w", ErrUpstreamResponse)
	}
	return PrimaryImage{Data: data, ContentType: contentType}, nil
}
