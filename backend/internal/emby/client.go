package emby

import (
	"bytes"
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
	"sync"
	"time"

	"media-hub/backend/internal/integration"
)

var (
	ErrNotConfigured    = errors.New("Emby is not configured")
	ErrMissingAPIKey    = errors.New("Emby API key is missing")
	ErrUnauthorized     = errors.New("Emby rejected authentication")
	ErrItemNotFound     = errors.New("Emby item was not found")
	ErrUpstreamResponse = errors.New("Emby returned an invalid response")
)

type clientConfig struct {
	baseURL string
	apiKey  string
	userID  string
}

type Client struct {
	mutex  sync.RWMutex
	config clientConfig
	client *http.Client
}

type ServerInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Library struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	CollectionType string `json:"collectionType,omitempty"`
}

type Item struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Year        int               `json:"year,omitempty"`
	ProviderIDs map[string]string `json:"providerIds,omitempty"`
	Season      int               `json:"season,omitempty"`
	Episode     int               `json:"episode,omitempty"`
}

type ItemDetail struct {
	Item
	OriginalTitle    string   `json:"originalTitle,omitempty"`
	Overview         string   `json:"overview,omitempty"`
	CommunityRating  float64  `json:"communityRating,omitempty"`
	RuntimeMinutes   int      `json:"runtimeMinutes,omitempty"`
	Genres           []string `json:"genres,omitempty"`
	MediaSourceCount int      `json:"mediaSourceCount"`
	ExternalURL      string   `json:"externalUrl"`
	AppURL           string   `json:"appUrl,omitempty"`
}

type SearchResult struct {
	Items []Item `json:"items"`
	Total int    `json:"total"`
}

type serverInfoResponse struct {
	ID               string `json:"Id"`
	ServerName       string `json:"ServerName"`
	Version          string `json:"Version"`
	LocalAddress     string `json:"LocalAddress"`
	WanAddress       string `json:"WanAddress"`
	WebSocketPort    int    `json:"WebSocketPortNumber"`
	CompletedStartup bool   `json:"CompletedStartup"`
}

type itemResponse struct {
	Items            []baseItem `json:"Items"`
	TotalRecordCount int        `json:"TotalRecordCount"`
}

type baseItem struct {
	ID                string            `json:"Id"`
	Name              string            `json:"Name"`
	OriginalTitle     string            `json:"OriginalTitle"`
	Overview          string            `json:"Overview"`
	Type              string            `json:"Type"`
	CollectionType    string            `json:"CollectionType"`
	ProductionYear    int               `json:"ProductionYear"`
	ProviderIDs       map[string]string `json:"ProviderIds"`
	ParentIndexNumber int               `json:"ParentIndexNumber"`
	IndexNumber       int               `json:"IndexNumber"`
	CommunityRating   float64           `json:"CommunityRating"`
	RunTimeTicks      int64             `json:"RunTimeTicks"`
	Genres            []string          `json:"Genres"`
	MediaSources      []mediaSource     `json:"MediaSources"`
}

type mediaSource struct {
	ID string `json:"Id"`
}

type playbackInfoResponse struct {
	MediaSources []mediaSource `json:"MediaSources"`
}

func NewClient(baseURL, apiKey string, timeout time.Duration, userID ...string) *Client {
	configuredUserID := ""
	if len(userID) > 0 {
		configuredUserID = strings.TrimSpace(userID[0])
	}
	return &Client{
		config: clientConfig{baseURL: baseURL, apiKey: apiKey, userID: configuredUserID},
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *Client) Configure(baseURL, apiKey, userID string) {
	c.mutex.Lock()
	c.config = clientConfig{baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), apiKey: strings.TrimSpace(apiKey), userID: strings.TrimSpace(userID)}
	c.mutex.Unlock()
}

func (c *Client) configuration() clientConfig {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.config
}

func (c *Client) Configured() bool {
	configuration := c.configuration()
	return configuration.baseURL != "" && configuration.apiKey != ""
}

func (c *Client) Check(ctx context.Context) integration.Health {
	health := integration.Health{ID: "emby", Label: "Emby"}
	configuration := c.configuration()
	if configuration.baseURL == "" {
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未配置服务地址"
		return health
	}

	var info ServerInfo
	var err error
	if configuration.apiKey == "" {
		info, err = c.readServerInfo(ctx, configuration, "System/Info/Public", false)
	} else {
		info, err = c.readServerInfo(ctx, configuration, "System/Info", true)
	}
	if err == nil {
		if configuration.apiKey == "" {
			health.Status = integration.StatusDegraded
			health.Detail = "Emby " + info.Version + " 在线，缺少 API Key"
		} else {
			health.Status = integration.StatusHealthy
			health.Detail = "Emby " + info.Version + " 在线"
		}
		return health
	}
	if errors.Is(err, ErrUnauthorized) {
		health.Status = integration.StatusDegraded
		health.Detail = "服务可达，但鉴权失败"
		return health
	}
	health.Status = integration.StatusUnavailable
	health.Detail = "无法读取 Emby 状态"
	return health
}

func (c *Client) PublicInfo(ctx context.Context) (ServerInfo, error) {
	configuration := c.configuration()
	if configuration.baseURL == "" {
		return ServerInfo{}, ErrNotConfigured
	}
	return c.readServerInfo(ctx, configuration, "System/Info/Public", false)
}

func (c *Client) ServerInfo(ctx context.Context) (ServerInfo, error) {
	configuration := c.configuration()
	if err := validateAuthenticated(configuration); err != nil {
		return ServerInfo{}, err
	}
	return c.readServerInfo(ctx, configuration, "System/Info", true)
}

func (c *Client) Libraries(ctx context.Context) ([]Library, error) {
	configuration := c.configuration()
	if err := validateAuthenticated(configuration); err != nil {
		return nil, err
	}
	query := url.Values{"Fields": {"CollectionType"}}
	var response itemResponse
	if err := c.getJSON(ctx, configuration, "Library/MediaFolders", query, true, &response); err != nil {
		return nil, err
	}

	libraries := make([]Library, 0, len(response.Items))
	for _, item := range response.Items {
		if item.ID == "" || item.Name == "" {
			continue
		}
		libraries = append(libraries, Library{
			ID: item.ID, Name: item.Name, CollectionType: item.CollectionType,
		})
	}
	return libraries, nil
}

func (c *Client) SearchItems(ctx context.Context, queryText string, limit int) (SearchResult, error) {
	return c.searchItems(ctx, c.configuration(), queryText, limit)
}

func (c *Client) searchItems(ctx context.Context, configuration clientConfig, queryText string, limit int) (SearchResult, error) {
	if err := validateAuthenticated(configuration); err != nil {
		return SearchResult{}, err
	}
	queryText = strings.TrimSpace(queryText)
	if queryText == "" {
		return SearchResult{}, fmt.Errorf("search term is required")
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	query := url.Values{
		"Fields":           {"ProviderIds"},
		"IncludeItemTypes": {"Movie,Series"},
		"Limit":            {strconv.Itoa(limit)},
		"Recursive":        {"true"},
		"SearchTerm":       {queryText},
	}
	if configuration.userID != "" {
		query.Set("UserId", configuration.userID)
	}
	var response itemResponse
	if err := c.getJSON(ctx, configuration, "Items", query, true, &response); err != nil {
		return SearchResult{}, err
	}
	return publicItems(response), nil
}

func (c *Client) BrowseItems(ctx context.Context, libraryID string, offset, limit int) (SearchResult, error) {
	configuration := c.configuration()
	if err := validateAuthenticated(configuration); err != nil {
		return SearchResult{}, err
	}
	libraryID = strings.TrimSpace(libraryID)
	if libraryID == "" {
		return SearchResult{}, ErrUpstreamResponse
	}
	if offset < 0 {
		offset = 0
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	query := url.Values{
		"Fields":           {"ProviderIds"},
		"IncludeItemTypes": {"Movie,Series"},
		"Limit":            {strconv.Itoa(limit)},
		"ParentId":         {libraryID},
		"Recursive":        {"true"},
		"SortBy":           {"SortName"},
		"SortOrder":        {"Ascending"},
		"StartIndex":       {strconv.Itoa(offset)},
	}
	if configuration.userID != "" {
		query.Set("UserId", configuration.userID)
	}
	var response itemResponse
	if err := c.getJSON(ctx, configuration, "Items", query, true, &response); err != nil {
		return SearchResult{}, err
	}
	return publicItems(response), nil
}

func (c *Client) ItemDetails(ctx context.Context, itemID string) (ItemDetail, error) {
	configuration := c.configuration()
	if err := validateAuthenticated(configuration); err != nil {
		return ItemDetail{}, err
	}
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return ItemDetail{}, ErrUpstreamResponse
	}
	query := url.Values{
		"Fields": {"CommunityRating,Genres,MediaSources,OriginalTitle,Overview,ProviderIds,RunTimeTicks"},
	}
	endpointPath := path.Join("Items", itemID)
	if configuration.userID != "" {
		endpointPath = path.Join("Users", configuration.userID, "Items", itemID)
	}
	var item baseItem
	if err := c.getJSONWithNotFound(ctx, configuration, endpointPath, query, true, &item); err != nil {
		return ItemDetail{}, err
	}
	if item.ID != itemID || item.Name == "" {
		return ItemDetail{}, ErrItemNotFound
	}
	externalURL, err := itemWebURL(configuration.baseURL, itemID)
	if err != nil {
		return ItemDetail{}, err
	}
	appURL := ""
	if serverInfo, infoErr := c.readServerInfo(ctx, configuration, "System/Info", true); infoErr == nil {
		appURL = itemAppURL(serverInfo.ID, itemID)
	}
	return ItemDetail{
		Item: publicItem(item), OriginalTitle: boundedText(item.OriginalTitle, 300),
		Overview: boundedText(item.Overview, 4000), CommunityRating: item.CommunityRating,
		RuntimeMinutes: int(item.RunTimeTicks / 600_000_000), Genres: boundedStrings(item.Genres, 32, 100),
		MediaSourceCount: len(item.MediaSources), ExternalURL: externalURL, AppURL: appURL,
	}, nil
}

func (c *Client) FindIndexedItem(ctx context.Context, title, mediaType string, year int, tmdbID string) (Item, bool, error) {
	return c.findIndexedItem(ctx, c.configuration(), title, mediaType, year, tmdbID)
}

func (c *Client) findIndexedItem(ctx context.Context, configuration clientConfig, title, mediaType string, year int, tmdbID string) (Item, bool, error) {
	expectedType := "Movie"
	if mediaType == "series" {
		expectedType = "Series"
	}
	if tmdbID != "" {
		query := url.Values{
			"AnyProviderIdEquals": {"Tmdb." + tmdbID},
			"Fields":              {"ProviderIds"},
			"IncludeItemTypes":    {expectedType},
			"Limit":               {"10"},
			"Recursive":           {"true"},
		}
		if configuration.userID != "" {
			query.Set("UserId", configuration.userID)
		}
		var response itemResponse
		if err := c.getJSON(ctx, configuration, "Items", query, true, &response); err != nil {
			return Item{}, false, err
		}
		for _, item := range response.Items {
			if item.ID != "" && item.Type == expectedType && item.ProviderIDs["Tmdb"] == tmdbID {
				return Item{
					ID: item.ID, Name: item.Name, Type: item.Type,
					Year: item.ProductionYear, ProviderIDs: item.ProviderIDs,
				}, true, nil
			}
		}
	}
	result, err := c.searchItems(ctx, configuration, title, 50)
	if err != nil {
		return Item{}, false, err
	}
	for _, item := range result.Items {
		if item.Type != expectedType {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(item.Name), strings.TrimSpace(title)) {
			continue
		}
		if year > 0 && item.Year > 0 && item.Year != year {
			continue
		}
		return item, true, nil
	}
	return Item{}, false, nil
}

func (c *Client) FindPlayableItem(ctx context.Context, title, mediaType string, year int, tmdbID string, season, episodeStart, episodeEnd int) (Item, bool, error) {
	configuration := c.configuration()
	item, found, err := c.findIndexedItem(ctx, configuration, title, mediaType, year, tmdbID)
	if err != nil || !found {
		return Item{}, found, err
	}
	if mediaType == "movie" {
		ready, err := c.playbackReady(ctx, configuration, item.ID)
		return item, ready, err
	}
	query := url.Values{
		"Fields":    {"ProviderIds,MediaSources"},
		"IsMissing": {"false"},
		"Limit":     {"10000"},
	}
	if season > 0 {
		query.Set("Season", strconv.Itoa(season))
	}
	if configuration.userID != "" {
		query.Set("UserId", configuration.userID)
	}
	var response itemResponse
	if err := c.getJSON(ctx, configuration, path.Join("Shows", item.ID, "Episodes"), query, true, &response); err != nil {
		return Item{}, false, err
	}
	playable := make(map[int]Item)
	for _, episode := range response.Items {
		if episode.Type != "Episode" || len(episode.MediaSources) == 0 {
			continue
		}
		if season > 0 && episode.ParentIndexNumber != season {
			continue
		}
		playable[episode.IndexNumber] = Item{
			ID: episode.ID, Name: episode.Name, Type: episode.Type, Year: episode.ProductionYear,
			Season: episode.ParentIndexNumber, Episode: episode.IndexNumber, ProviderIDs: episode.ProviderIDs,
		}
	}
	if episodeStart > 0 {
		if episodeEnd < episodeStart {
			return Item{}, false, ErrUpstreamResponse
		}
		for number := episodeStart; number <= episodeEnd; number++ {
			if _, ok := playable[number]; !ok {
				return Item{}, false, nil
			}
		}
		return item, true, nil
	}
	for range playable {
		return item, true, nil
	}
	return Item{}, false, nil
}

func (c *Client) RefreshLibrary(ctx context.Context, libraryID string) error {
	return c.refreshItem(ctx, libraryID)
}

func (c *Client) RefreshItem(ctx context.Context, itemID string) error {
	return c.refreshItem(ctx, itemID)
}

func (c *Client) refreshItem(ctx context.Context, itemID string) error {
	configuration := c.configuration()
	if err := validateAuthenticated(configuration); err != nil {
		return err
	}
	if strings.TrimSpace(itemID) == "" {
		return ErrUpstreamResponse
	}
	return c.postJSON(ctx, configuration, path.Join("Items", itemID, "Refresh"), nil, nil)
}

func (c *Client) PlaybackReady(ctx context.Context, itemID string) (bool, error) {
	return c.playbackReady(ctx, c.configuration(), itemID)
}

func (c *Client) playbackReady(ctx context.Context, configuration clientConfig, itemID string) (bool, error) {
	if err := validateAuthenticated(configuration); err != nil {
		return false, err
	}
	if strings.TrimSpace(itemID) == "" {
		return false, ErrUpstreamResponse
	}
	query := url.Values{}
	if configuration.userID != "" {
		query.Set("UserId", configuration.userID)
	}
	var response playbackInfoResponse
	if err := c.postJSON(ctx, configuration, path.Join("Items", itemID, "PlaybackInfo"), query, &response); err != nil {
		return false, err
	}
	return len(response.MediaSources) > 0, nil
}

func (c *Client) readServerInfo(ctx context.Context, configuration clientConfig, endpointPath string, authenticated bool) (ServerInfo, error) {
	var response serverInfoResponse
	if err := c.getJSON(ctx, configuration, endpointPath, nil, authenticated, &response); err != nil {
		return ServerInfo{}, err
	}
	if response.ID == "" || response.Version == "" {
		return ServerInfo{}, ErrUpstreamResponse
	}
	return ServerInfo{ID: response.ID, Name: response.ServerName, Version: response.Version}, nil
}

func publicItems(response itemResponse) SearchResult {
	items := make([]Item, 0, len(response.Items))
	for _, item := range response.Items {
		if item.ID == "" || item.Name == "" {
			continue
		}
		items = append(items, publicItem(item))
	}
	return SearchResult{Items: items, Total: response.TotalRecordCount}
}

func publicItem(item baseItem) Item {
	return Item{
		ID: item.ID, Name: item.Name, Type: item.Type, Year: item.ProductionYear,
		ProviderIDs: item.ProviderIDs, Season: item.ParentIndexNumber, Episode: item.IndexNumber,
	}
}

func itemWebURL(baseURL, itemID string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", ErrUpstreamResponse
	}
	parsed.Path = path.Join(parsed.Path, "web/index.html")
	parsed.RawQuery = ""
	parsed.Fragment = "!/item?id=" + url.QueryEscape(itemID)
	return parsed.String(), nil
}

func itemAppURL(serverID, itemID string) string {
	serverID = strings.TrimSpace(serverID)
	itemID = strings.TrimSpace(itemID)
	if !validEmbyIdentifier(serverID) || !validEmbyIdentifier(itemID) {
		return ""
	}
	return (&url.URL{Scheme: "emby", Host: "items", Path: "/" + serverID + "/" + itemID}).String()
}

func validEmbyIdentifier(value string) bool {
	if len(value) < 1 || len(value) > 128 {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

func boundedText(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}

func boundedStrings(values []string, countLimit, lengthLimit int) []string {
	if len(values) > countLimit {
		values = values[:countLimit]
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = boundedText(value, lengthLimit); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func validateAuthenticated(configuration clientConfig) error {
	if configuration.baseURL == "" {
		return ErrNotConfigured
	}
	if configuration.apiKey == "" {
		return ErrMissingAPIKey
	}
	return nil
}

func (c *Client) getJSON(
	ctx context.Context,
	configuration clientConfig,
	endpointPath string,
	query url.Values,
	authenticated bool,
	target any,
) error {
	return c.getJSONResponse(ctx, configuration, endpointPath, query, authenticated, false, target)
}

func (c *Client) getJSONWithNotFound(
	ctx context.Context,
	configuration clientConfig,
	endpointPath string,
	query url.Values,
	authenticated bool,
	target any,
) error {
	return c.getJSONResponse(ctx, configuration, endpointPath, query, authenticated, true, target)
}

func (c *Client) getJSONResponse(
	ctx context.Context,
	configuration clientConfig,
	endpointPath string,
	query url.Values,
	authenticated bool,
	mapNotFound bool,
	target any,
) error {
	endpoint, err := endpointURL(configuration.baseURL, endpointPath, query)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create Emby request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Media-Hub/emby")
	if authenticated {
		request.Header.Set("X-Emby-Token", configuration.apiKey)
	}

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

	decoder := json.NewDecoder(io.LimitReader(response.Body, 4<<20))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode Emby response: %w", err)
	}
	return nil
}

func (c *Client) postJSON(ctx context.Context, configuration clientConfig, endpointPath string, query url.Values, target any) error {
	endpoint, err := endpointURL(configuration.baseURL, endpointPath, query)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBufferString("{}"))
	if err != nil {
		return fmt.Errorf("create Emby request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Media-Hub/emby")
	request.Header.Set("X-Emby-Token", configuration.apiKey)
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("request Emby: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
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

func endpointURL(baseURL, endpointPath string, query url.Values) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse Emby URL: %w", err)
	}
	parsed.Path = path.Join("/", parsed.Path, endpointPath)
	values := parsed.Query()
	for key, items := range query {
		for _, item := range items {
			values.Add(key, item)
		}
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}
