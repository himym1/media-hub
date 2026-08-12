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
	"time"

	"media-hub/backend/internal/integration"
)

var (
	ErrNotConfigured    = errors.New("Emby is not configured")
	ErrMissingAPIKey    = errors.New("Emby API key is missing")
	ErrUnauthorized     = errors.New("Emby rejected authentication")
	ErrUpstreamResponse = errors.New("Emby returned an invalid response")
)

type Client struct {
	baseURL string
	apiKey  string
	userID  string
	client  *http.Client
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
	Type              string            `json:"Type"`
	CollectionType    string            `json:"CollectionType"`
	ProductionYear    int               `json:"ProductionYear"`
	ProviderIDs       map[string]string `json:"ProviderIds"`
	ParentIndexNumber int               `json:"ParentIndexNumber"`
	IndexNumber       int               `json:"IndexNumber"`
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
		baseURL: baseURL,
		apiKey:  apiKey,
		userID:  configuredUserID,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *Client) Configured() bool {
	return c.baseURL != "" && c.apiKey != ""
}

func (c *Client) Check(ctx context.Context) integration.Health {
	health := integration.Health{ID: "emby", Label: "Emby"}
	if c.baseURL == "" {
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未配置服务地址"
		return health
	}

	var info ServerInfo
	var err error
	if c.apiKey == "" {
		info, err = c.PublicInfo(ctx)
	} else {
		info, err = c.ServerInfo(ctx)
	}
	if err == nil {
		if c.apiKey == "" {
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
	if c.baseURL == "" {
		return ServerInfo{}, ErrNotConfigured
	}
	return c.readServerInfo(ctx, "System/Info/Public", false)
}

func (c *Client) ServerInfo(ctx context.Context) (ServerInfo, error) {
	if err := c.validateAuthenticated(); err != nil {
		return ServerInfo{}, err
	}
	return c.readServerInfo(ctx, "System/Info", true)
}

func (c *Client) Libraries(ctx context.Context) ([]Library, error) {
	if err := c.validateAuthenticated(); err != nil {
		return nil, err
	}
	query := url.Values{"Fields": {"CollectionType"}}
	var response itemResponse
	if err := c.getJSON(ctx, "Library/MediaFolders", query, true, &response); err != nil {
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
	if err := c.validateAuthenticated(); err != nil {
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
	var response itemResponse
	if err := c.getJSON(ctx, "Items", query, true, &response); err != nil {
		return SearchResult{}, err
	}

	items := make([]Item, 0, len(response.Items))
	for _, item := range response.Items {
		if item.ID == "" || item.Name == "" {
			continue
		}
		items = append(items, Item{
			ID: item.ID, Name: item.Name, Type: item.Type,
			Year: item.ProductionYear, ProviderIDs: item.ProviderIDs,
		})
	}
	return SearchResult{Items: items, Total: response.TotalRecordCount}, nil
}

func (c *Client) FindIndexedItem(ctx context.Context, title, mediaType string, year int, tmdbID string) (Item, bool, error) {
	result, err := c.SearchItems(ctx, title, 50)
	if err != nil {
		return Item{}, false, err
	}
	expectedType := "Movie"
	if mediaType == "series" {
		expectedType = "Series"
	}
	for _, item := range result.Items {
		if item.Type != expectedType {
			continue
		}
		if tmdbID != "" && item.ProviderIDs["Tmdb"] == tmdbID {
			return item, true, nil
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
	item, found, err := c.FindIndexedItem(ctx, title, mediaType, year, tmdbID)
	if err != nil || !found {
		return Item{}, found, err
	}
	if mediaType == "movie" {
		ready, err := c.PlaybackReady(ctx, item.ID)
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
	if c.userID != "" {
		query.Set("UserId", c.userID)
	}
	var response itemResponse
	if err := c.getJSON(ctx, path.Join("Shows", item.ID, "Episodes"), query, true, &response); err != nil {
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
	if err := c.validateAuthenticated(); err != nil {
		return err
	}
	if strings.TrimSpace(libraryID) == "" {
		return ErrUpstreamResponse
	}
	return c.postJSON(ctx, path.Join("Items", libraryID, "Refresh"), nil, nil)
}

func (c *Client) PlaybackReady(ctx context.Context, itemID string) (bool, error) {
	if err := c.validateAuthenticated(); err != nil {
		return false, err
	}
	if strings.TrimSpace(itemID) == "" {
		return false, ErrUpstreamResponse
	}
	query := url.Values{}
	if c.userID != "" {
		query.Set("UserId", c.userID)
	}
	var response playbackInfoResponse
	if err := c.postJSON(ctx, path.Join("Items", itemID, "PlaybackInfo"), query, &response); err != nil {
		return false, err
	}
	return len(response.MediaSources) > 0, nil
}

func (c *Client) readServerInfo(ctx context.Context, endpointPath string, authenticated bool) (ServerInfo, error) {
	var response serverInfoResponse
	if err := c.getJSON(ctx, endpointPath, nil, authenticated, &response); err != nil {
		return ServerInfo{}, err
	}
	if response.ID == "" || response.Version == "" {
		return ServerInfo{}, ErrUpstreamResponse
	}
	return ServerInfo{ID: response.ID, Name: response.ServerName, Version: response.Version}, nil
}

func (c *Client) validateAuthenticated() error {
	if c.baseURL == "" {
		return ErrNotConfigured
	}
	if c.apiKey == "" {
		return ErrMissingAPIKey
	}
	return nil
}

func (c *Client) getJSON(
	ctx context.Context,
	endpointPath string,
	query url.Values,
	authenticated bool,
	target any,
) error {
	endpoint, err := c.endpoint(endpointPath, query)
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
		request.Header.Set("X-Emby-Token", c.apiKey)
	}

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

	decoder := json.NewDecoder(io.LimitReader(response.Body, 4<<20))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode Emby response: %w", err)
	}
	return nil
}

func (c *Client) postJSON(ctx context.Context, endpointPath string, query url.Values, target any) error {
	endpoint, err := c.endpoint(endpointPath, query)
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
	request.Header.Set("X-Emby-Token", c.apiKey)
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

func (c *Client) endpoint(endpointPath string, query url.Values) (string, error) {
	parsed, err := url.Parse(c.baseURL)
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
