package emby

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"media-hub/backend/internal/integration"
	"media-hub/backend/internal/playback"
)

const (
	sharedIDPrefix     = "r_"
	sharedClientName   = "Emby Web"
	sharedClientDevice = "Chrome"
	sharedClientVer    = "4.9.5.0"
)

var ErrSharedReadOnly = errors.New("shared Emby is read-only")

func IsSharedID(id string) bool {
	return strings.HasPrefix(strings.TrimSpace(id), sharedIDPrefix)
}

func sharedPublicID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" || IsSharedID(id) {
		return id
	}
	return sharedIDPrefix + id
}

func sharedNativeID(id string) (string, bool) {
	id = strings.TrimSpace(id)
	if !IsSharedID(id) {
		return id, false
	}
	native := strings.TrimPrefix(id, sharedIDPrefix)
	if !validEmbyIdentifier(native) {
		return "", false
	}
	return native, true
}

func (c *Client) SharedConfigured() bool {
	configuration := c.configuration()
	return configuration.shared && configuration.baseURL != "" && configuration.username != "" && configuration.password != ""
}

func (c *Client) CheckShared(ctx context.Context) integration.Health {
	health := integration.Health{ID: "shared-emby", Label: "共享 Emby"}
	configuration := c.configuration()
	if !configuration.shared {
		health.Status = integration.StatusUnconfigured
		health.Detail = "未启用"
		return health
	}
	if configuration.baseURL == "" {
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未配置服务地址"
		return health
	}
	if configuration.username == "" || configuration.password == "" {
		health.Status = integration.StatusDegraded
		health.Detail = "缺少用户名或密码"
		return health
	}
	if err := c.ensureSharedSession(ctx); err != nil {
		if errors.Is(err, ErrUnauthorized) {
			health.Status = integration.StatusDegraded
			health.Detail = "服务可达，但鉴权失败"
			return health
		}
		health.Status = integration.StatusUnavailable
		health.Detail = "无法登录共享 Emby"
		return health
	}
	info, err := c.readServerInfo(ctx, c.configuration(), "System/Info", true)
	if err != nil {
		health.Status = integration.StatusUnavailable
		health.Detail = "无法读取 Emby 状态"
		return health
	}
	health.Status = integration.StatusHealthy
	health.Detail = "Emby " + info.Version + " 在线"
	return health
}

func (c *Client) ensureSharedSession(ctx context.Context) error {
	configuration := c.configuration()
	if !configuration.shared {
		return ErrNotConfigured
	}
	if configuration.baseURL == "" || configuration.username == "" || configuration.password == "" {
		return ErrNotConfigured
	}
	if configuration.apiKey != "" && configuration.userID != "" {
		return nil
	}
	return c.loginShared(ctx)
}

func (c *Client) loginShared(ctx context.Context) error {
	configuration := c.configuration()
	payload, err := json.Marshal(map[string]string{
		"Username": configuration.username,
		"Pw":       configuration.password,
	})
	if err != nil {
		return fmt.Errorf("encode shared Emby authentication: %w", err)
	}
	endpoint, err := endpointURL(configuration.baseURL, "Users/AuthenticateByName", nil)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("create shared Emby authentication: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", sharedClientName+"/"+sharedClientVer)
	applyNamedEmbyClientAuth(request, sharedClientName, sharedClientDevice, sharedClientVer)
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("request shared Emby: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return ErrUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return ErrUpstreamResponse
	}
	var body struct {
		AccessToken string `json:"AccessToken"`
		User        struct {
			ID   string `json:"Id"`
			Name string `json:"Name"`
		} `json:"User"`
	}
	if err := decodeEmbyBody(response, &body); err != nil {
		return err
	}
	token := strings.TrimSpace(body.AccessToken)
	userID := strings.TrimSpace(body.User.ID)
	if token == "" || userID == "" {
		return ErrUnauthorized
	}
	c.mutex.Lock()
	c.config.apiKey = token
	c.config.userID = userID
	c.sessionToken = token
	c.mutex.Unlock()
	return nil
}

func (c *Client) clearSharedAuth() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if !c.config.shared {
		return
	}
	c.config.apiKey = ""
	c.config.userID = ""
	c.sessionToken = ""
}

func (c *Client) withSharedAuth(ctx context.Context, run func(clientConfig) error) error {
	if err := c.ensureSharedSession(ctx); err != nil {
		return err
	}
	err := run(c.configuration())
	if !errors.Is(err, ErrUnauthorized) {
		return err
	}
	c.clearSharedAuth()
	if err := c.ensureSharedSession(ctx); err != nil {
		return err
	}
	return run(c.configuration())
}

func (c *Client) sharedLibraries(ctx context.Context) ([]Library, error) {
	var libraries []Library
	err := c.withSharedAuth(ctx, func(configuration clientConfig) error {
		var response itemResponse
		if err := c.getJSON(ctx, configuration, path.Join("Users", configuration.userID, "Views"), nil, true, &response); err != nil {
			return err
		}
		next := make([]Library, 0, len(response.Items))
		for _, item := range response.Items {
			if item.ID == "" || item.Name == "" {
				continue
			}
			collectionType := strings.TrimSpace(item.CollectionType)
			switch collectionType {
			case "movies", "tvshows", "boxsets", "":
			default:
				continue
			}
			next = append(next, Library{
				ID:             sharedPublicID(item.ID),
				Name:           "共享/" + item.Name,
				CollectionType: collectionType,
			})
		}
		libraries = next
		return nil
	})
	return libraries, err
}

func (c *Client) sharedBrowseItems(ctx context.Context, libraryID string, offset, limit int) (SearchResult, error) {
	nativeID, ok := sharedNativeID(libraryID)
	if !ok {
		return SearchResult{}, ErrItemNotFound
	}
	if offset < 0 {
		offset = 0
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	var result SearchResult
	err := c.withSharedAuth(ctx, func(configuration clientConfig) error {
		query := url.Values{
			"Fields":           {"ProviderIds,UserData,MediaSources,Path"},
			"IncludeItemTypes": {"Movie,Series"},
			"Limit":            {strconv.Itoa(limit)},
			"ParentId":         {nativeID},
			"Recursive":        {"true"},
			"SortBy":           {"SortName"},
			"SortOrder":        {"Ascending"},
			"StartIndex":       {strconv.Itoa(offset)},
			"UserId":           {configuration.userID},
		}
		var response itemResponse
		if err := c.getJSON(ctx, configuration, "Items", query, true, &response); err != nil {
			return err
		}
		result = prefixSharedSearch(publicItems(response))
		return nil
	})
	return result, err
}

func (c *Client) sharedSearchItems(ctx context.Context, queryText string, limit int) (SearchResult, error) {
	queryText = strings.TrimSpace(queryText)
	if queryText == "" {
		return SearchResult{}, fmt.Errorf("search term is required")
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	var result SearchResult
	err := c.withSharedAuth(ctx, func(configuration clientConfig) error {
		query := url.Values{
			"Fields":           {"ProviderIds,MediaSources,Path"},
			"IncludeItemTypes": {"Movie,Series"},
			"Limit":            {strconv.Itoa(limit)},
			"Recursive":        {"true"},
			"SearchTerm":       {queryText},
			"UserId":           {configuration.userID},
		}
		var response itemResponse
		if err := c.getJSON(ctx, configuration, "Items", query, true, &response); err != nil {
			return err
		}
		result = prefixSharedSearch(publicItems(response))
		return nil
	})
	return result, err
}

func (c *Client) sharedItemDetails(ctx context.Context, itemID string) (ItemDetail, error) {
	nativeID, ok := sharedNativeID(itemID)
	if !ok {
		return ItemDetail{}, ErrItemNotFound
	}
	var detail ItemDetail
	err := c.withSharedAuth(ctx, func(configuration clientConfig) error {
		query := url.Values{
			"Fields": {"CommunityRating,Genres,MediaSources,OriginalTitle,Overview,Path,ProviderIds,RunTimeTicks,SeriesName,UserData"},
		}
		var item baseItem
		if err := c.getJSONWithNotFound(ctx, configuration, path.Join("Users", configuration.userID, "Items", nativeID), query, true, &item); err != nil {
			return err
		}
		if item.ID != nativeID || item.Name == "" {
			return ErrItemNotFound
		}
		detail = ItemDetail{
			Item:             publicItem(item),
			OriginalTitle:    boundedText(item.OriginalTitle, 300),
			Overview:         boundedText(item.Overview, 4000),
			CommunityRating:  item.CommunityRating,
			RuntimeMinutes:   int(item.RunTimeTicks / 600_000_000),
			Genres:           boundedStrings(item.Genres, 32, 100),
			MediaSourceCount: len(item.MediaSources),
			TechSpecs:        extractMediaTechSpecs(item.MediaSources),
		}
		detail.ID = sharedPublicID(detail.ID)
		return nil
	})
	return detail, err
}

func (c *Client) sharedEpisodes(ctx context.Context, seriesID string) ([]Episode, error) {
	nativeID, ok := sharedNativeID(seriesID)
	if !ok {
		return nil, ErrItemNotFound
	}
	var episodes []Episode
	err := c.withSharedAuth(ctx, func(configuration clientConfig) error {
		query := url.Values{
			"Fields":           {"ProviderIds,UserData,MediaSources,Path"},
			"IncludeItemTypes": {"Episode"},
			"ParentId":         {nativeID},
			"Recursive":        {"true"},
			"SortBy":           {"SortName"},
			"SortOrder":        {"Ascending"},
			"UserId":           {configuration.userID},
		}
		var response itemResponse
		if err := c.getJSON(ctx, configuration, "Items", query, true, &response); err != nil {
			return err
		}
		next := make([]Episode, 0, len(response.Items))
		for _, item := range response.Items {
			if item.Type != "Episode" || item.ID == "" || item.Name == "" {
				continue
			}
			next = append(next, Episode{
				Item: Item{
					ID:                 sharedPublicID(item.ID),
					Name:               item.Name,
					Type:               item.Type,
					Year:               item.ProductionYear,
					Season:             item.ParentIndexNumber,
					Episode:            item.IndexNumber,
					PlaybackPositionMS: item.UserData.PlaybackPositionTicks / 10_000,
					Played:             item.UserData.Played,
				},
				TechSpecs: extractMediaTechSpecs(item.MediaSources),
			})
		}
		episodes = next
		return nil
	})
	return episodes, err
}

func (c *Client) sharedPrimaryImage(ctx context.Context, itemID string, maxWidth int) (PrimaryImage, error) {
	nativeID, ok := sharedNativeID(itemID)
	if !ok {
		return PrimaryImage{}, ErrItemNotFound
	}
	var image PrimaryImage
	err := c.withSharedAuth(ctx, func(configuration clientConfig) error {
		var err error
		image, err = c.primaryImageWith(ctx, configuration, nativeID, maxWidth)
		return err
	})
	return image, err
}

func (c *Client) sharedBackdropImage(ctx context.Context, itemID string, maxWidth int) (PrimaryImage, error) {
	nativeID, ok := sharedNativeID(itemID)
	if !ok {
		return PrimaryImage{}, ErrItemNotFound
	}
	var image PrimaryImage
	err := c.withSharedAuth(ctx, func(configuration clientConfig) error {
		var err error
		image, err = c.backdropImageWith(ctx, configuration, nativeID, maxWidth)
		return err
	})
	return image, err
}

func (c *Client) ResolveSharedItem(ctx context.Context, target playback.EmbyItemTarget, playbackUserAgent string) (playback.SourceMedia, error) {
	nativeID, ok := sharedNativeID(target.ItemID)
	if !ok {
		return playback.SourceMedia{}, playback.ErrNotFound
	}
	if strings.TrimSpace(playbackUserAgent) == "" {
		return playback.SourceMedia{}, playback.ErrInvalidRequest
	}
	var media playback.SourceMedia
	err := c.withSharedAuth(ctx, func(configuration clientConfig) error {
		var item baseItem
		if err := c.getJSONWithNotFound(ctx, configuration, path.Join("Users", configuration.userID, "Items", nativeID), url.Values{
			"Fields": {"MediaSources,UserData"},
		}, true, &item); err != nil {
			return err
		}
		if item.ID != nativeID || item.Name == "" || !isPlayableItemType(item.Type) {
			return playback.ErrNotFound
		}
		source, streamURL, err := c.sharedStreamURL(ctx, configuration, item)
		if err != nil {
			return err
		}
		session, err := c.playbackSession(nativeID, source.ID, "shared-"+nativeID)
		if err != nil {
			return playback.ErrUnavailable
		}
		media = playback.SourceMedia{
			Name:            item.Name,
			URL:             streamURL,
			Session:         session,
			StartPositionMS: max(0, item.UserData.PlaybackPositionTicks/10_000),
		}
		return nil
	})
	if err != nil {
		return playback.SourceMedia{}, normalizePlaybackError(err)
	}
	return media, nil
}

func (c *Client) sharedStreamURL(ctx context.Context, configuration clientConfig, item baseItem) (mediaSource, string, error) {
	sources := item.MediaSources
	if len(sources) == 0 {
		sources = []mediaSource{{ID: item.ID, Container: "mkv", SupportsDirectPlay: true}}
	}
	for _, source := range sources {
		if !validEmbyIdentifier(source.ID) {
			continue
		}
		if direct := absoluteEmbyURL(configuration.baseURL, source.DirectStreamURL); direct != "" {
			if withToken := ensureEmbyTokenQuery(direct, configuration.apiKey); withToken != "" {
				return source, withToken, nil
			}
		}
		container := strings.Trim(strings.ToLower(source.Container), ".")
		endpointPath := path.Join("Videos", item.ID, "stream")
		if container != "" && len(container) <= 16 {
			endpointPath += "." + container
		}
		query := url.Values{
			"Static":            {"true"},
			"MediaSourceId":     {source.ID},
			"EnableRedirection": {"true"},
			"EnableRemoteMedia": {"true"},
			"api_key":           {configuration.apiKey},
		}
		endpoint, err := endpointURL(configuration.baseURL, endpointPath, query)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(endpoint, "https://") {
			continue
		}
		// Probe that the stream endpoint answers with media (or accept) under the shared session.
		if err := c.probeSharedStream(ctx, endpoint, configuration.apiKey); err != nil {
			continue
		}
		return source, endpoint, nil
	}
	return mediaSource{}, "", playback.ErrUnavailable
}

func (c *Client) probeSharedStream(ctx context.Context, endpoint, token string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Range", "bytes=0-0")
	request.Header.Set("Accept", "*/*")
	request.Header.Set("User-Agent", sharedClientName+"/"+sharedClientVer)
	request.Header.Set("X-Emby-Token", token)
	applyNamedEmbyClientAuth(request, sharedClientName, sharedClientDevice, sharedClientVer)
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64))
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	switch response.StatusCode {
	case http.StatusOK, http.StatusPartialContent,
		http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return nil
	default:
		return ErrUpstreamResponse
	}
}

func absoluteEmbyURL(baseURL, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://") {
		return value
	}
	if strings.HasPrefix(value, "/") {
		return strings.TrimRight(baseURL, "/") + value
	}
	return ""
}

func ensureEmbyTokenQuery(rawURL, token string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return ""
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	query := parsed.Query()
	if query.Get("api_key") == "" && query.Get("ApiKey") == "" {
		query.Set("api_key", token)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func prefixSharedSearch(result SearchResult) SearchResult {
	for index := range result.Items {
		result.Items[index].ID = sharedPublicID(result.Items[index].ID)
	}
	return result
}

func applyNamedEmbyClientAuth(request *http.Request, client, device, version string) {
	authorization := strings.Join([]string{
		`MediaBrowser Client="` + client + `"`,
		`Device="` + device + `"`,
		`DeviceId="media-hub-shared"`,
		`Version="` + version + `"`,
	}, ", ")
	request.Header.Set("X-Emby-Authorization", authorization)
	request.Header.Set("Authorization", authorization)
}
