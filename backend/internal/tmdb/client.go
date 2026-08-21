package tmdb

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
	"sync"
	"time"

	"media-hub/backend/internal/integration"
	"media-hub/backend/internal/search"
)

var (
	ErrNotConfigured    = errors.New("TMDB is not configured")
	ErrUnauthorized     = errors.New("TMDB rejected authentication")
	ErrUpstreamResponse = errors.New("TMDB returned an invalid response")
)

type clientConfig struct {
	baseURL string
	token   string
}

type Client struct {
	mutex  sync.RWMutex
	config clientConfig
	client *http.Client
	cache  map[string]discoveryCacheEntry
}

type discoveryCacheEntry struct {
	items   []DiscoveryItem
	expires time.Time
}

const discoveryCacheTTL = 5 * time.Minute

type multiSearchResponse struct {
	Results []struct {
		ID            int64  `json:"id"`
		MediaType     string `json:"media_type"`
		Title         string `json:"title"`
		OriginalTitle string `json:"original_title"`
		Name          string `json:"name"`
		OriginalName  string `json:"original_name"`
		ReleaseDate   string `json:"release_date"`
		FirstAirDate  string `json:"first_air_date"`
		PosterPath    string `json:"poster_path"`
	} `json:"results"`
}

type DiscoveryItem struct {
	TMDBID    string `json:"tmdbId"`
	Title     string `json:"title"`
	Year      int    `json:"year"`
	MediaType string `json:"mediaType"`
	PosterURL string `json:"posterUrl,omitempty"`
}

func NewClient(baseURL, token string, timeout time.Duration) *Client {
	return &Client{
		config: clientConfig{baseURL: strings.TrimRight(baseURL, "/"), token: token},
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		cache: map[string]discoveryCacheEntry{},
	}
}

func (c *Client) Configure(baseURL, token string) {
	c.mutex.Lock()
	c.config = clientConfig{baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), token: strings.TrimSpace(token)}
	c.cache = map[string]discoveryCacheEntry{}
	c.mutex.Unlock()
}

func (c *Client) configuration() clientConfig {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.config
}

func (c *Client) Check(ctx context.Context) integration.Health {
	health := integration.Health{ID: "tmdb", Label: "TMDB"}
	configuration := c.configuration()
	if configuration.baseURL == "" || configuration.token == "" {
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未配置身份匹配"
		return health
	}
	var response struct {
		Images json.RawMessage `json:"images"`
	}
	if err := c.getJSON(ctx, "configuration", nil, &response); err != nil {
		if errors.Is(err, ErrUnauthorized) {
			health.Status = integration.StatusDegraded
			health.Detail = "服务可达，但鉴权失败"
			return health
		}
		health.Status = integration.StatusUnavailable
		health.Detail = "无法读取 TMDB 状态"
		return health
	}
	health.Status = integration.StatusHealthy
	health.Detail = "影视身份匹配可用"
	return health
}

func (c *Client) Resolve(ctx context.Context, queryText string) ([]search.Identity, error) {
	configuration := c.configuration()
	if configuration.baseURL == "" || configuration.token == "" {
		return nil, ErrNotConfigured
	}
	queryText = strings.TrimSpace(queryText)
	if queryText == "" {
		return []search.Identity{}, nil
	}
	query := url.Values{
		"include_adult": {"false"},
		"language":      {"zh-CN"},
		"query":         {queryText},
	}
	var response multiSearchResponse
	if err := c.getJSON(ctx, "search/multi", query, &response); err != nil {
		return nil, err
	}
	identities := make([]search.Identity, 0, len(response.Results))
	for _, result := range response.Results {
		identity := search.Identity{TMDBID: strconv.FormatInt(result.ID, 10)}
		switch result.MediaType {
		case "movie":
			identity.MediaType = "movie"
			identity.Title = result.Title
			identity.OriginalTitle = result.OriginalTitle
			identity.Year = year(result.ReleaseDate)
		case "tv":
			identity.MediaType = "series"
			identity.Title = result.Name
			identity.OriginalTitle = result.OriginalName
			identity.Year = year(result.FirstAirDate)
		default:
			continue
		}
		if result.ID < 1 || strings.TrimSpace(identity.Title) == "" {
			continue
		}
		if strings.HasPrefix(result.PosterPath, "/") {
			identity.PosterURL = "https://image.tmdb.org/t/p/w500" + result.PosterPath
		}
		identities = append(identities, identity)
	}
	return identities, nil
}

func (c *Client) Trending(ctx context.Context, mediaType string, limit int) ([]DiscoveryItem, error) {
	if mediaType != "all" && mediaType != "movie" && mediaType != "series" {
		return nil, ErrUpstreamResponse
	}
	endpointType := mediaType
	if endpointType == "series" {
		endpointType = "tv"
	}
	return c.discovery(ctx, path.Join("trending", endpointType, "week"), mediaType, limit)
}

func (c *Client) Recommendations(ctx context.Context, mediaType, tmdbID string, limit int) ([]DiscoveryItem, error) {
	if mediaType != "movie" && mediaType != "series" {
		return nil, ErrUpstreamResponse
	}
	if parsed, err := strconv.ParseInt(tmdbID, 10, 64); err != nil || parsed < 1 {
		return nil, ErrUpstreamResponse
	}
	endpointType := mediaType
	if endpointType == "series" {
		endpointType = "tv"
	}
	return c.discovery(ctx, path.Join(endpointType, tmdbID, "recommendations"), mediaType, limit)
}

func (c *Client) discovery(ctx context.Context, endpointPath, fallbackMediaType string, limit int) ([]DiscoveryItem, error) {
	configuration := c.configuration()
	if configuration.baseURL == "" || configuration.token == "" {
		return nil, ErrNotConfigured
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 20 {
		limit = 20
	}
	cacheKey := endpointPath + "|limit=" + strconv.Itoa(limit)
	if items, ok := c.cachedDiscovery(cacheKey); ok {
		return items, nil
	}
	query := url.Values{"include_adult": {"false"}, "language": {"zh-CN"}}
	var response multiSearchResponse
	if err := c.getJSON(ctx, endpointPath, query, &response); err != nil {
		return nil, err
	}
	items := make([]DiscoveryItem, 0, min(limit, len(response.Results)))
	for _, result := range response.Results {
		mediaType := fallbackMediaType
		if result.MediaType != "" {
			mediaType = result.MediaType
		}
		item := discoveryItem(result.ID, mediaType, result.Title, result.Name, result.ReleaseDate, result.FirstAirDate, result.PosterPath)
		if item.TMDBID == "" {
			continue
		}
		items = append(items, item)
		if len(items) == limit {
			break
		}
	}
	c.storeDiscovery(cacheKey, items)
	return items, nil
}

func (c *Client) cachedDiscovery(key string) ([]DiscoveryItem, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	entry, ok := c.cache[key]
	if !ok || time.Now().After(entry.expires) {
		return nil, false
	}
	return cloneDiscoveryItems(entry.items), true
}

func (c *Client) storeDiscovery(key string, items []DiscoveryItem) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache[key] = discoveryCacheEntry{
		items:   cloneDiscoveryItems(items),
		expires: time.Now().Add(discoveryCacheTTL),
	}
}

func cloneDiscoveryItems(items []DiscoveryItem) []DiscoveryItem {
	if len(items) == 0 {
		return []DiscoveryItem{}
	}
	cloned := make([]DiscoveryItem, len(items))
	copy(cloned, items)
	return cloned
}

func discoveryItem(id int64, mediaType, movieTitle, seriesTitle, releaseDate, firstAirDate, posterPath string) DiscoveryItem {
	if id < 1 {
		return DiscoveryItem{}
	}
	item := DiscoveryItem{TMDBID: strconv.FormatInt(id, 10)}
	switch mediaType {
	case "movie":
		item.MediaType, item.Title, item.Year = "movie", movieTitle, year(releaseDate)
	case "tv", "series":
		item.MediaType, item.Title, item.Year = "series", seriesTitle, year(firstAirDate)
	default:
		return DiscoveryItem{}
	}
	if strings.TrimSpace(item.Title) == "" {
		return DiscoveryItem{}
	}
	if strings.HasPrefix(posterPath, "/") {
		item.PosterURL = "https://image.tmdb.org/t/p/w500" + posterPath
	}
	return item
}

func (c *Client) getJSON(ctx context.Context, endpointPath string, query url.Values, target any) error {
	configuration := c.configuration()
	parsed, err := url.Parse(configuration.baseURL)
	if err != nil {
		return fmt.Errorf("parse TMDB URL: %w", err)
	}
	parsed.Path = path.Join("/", parsed.Path, endpointPath)
	values := parsed.Query()
	for key, items := range query {
		for _, item := range items {
			values.Add(key, item)
		}
	}
	parsed.RawQuery = values.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return fmt.Errorf("create TMDB request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+configuration.token)
	request.Header.Set("User-Agent", "Media-Hub/tmdb")
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("request TMDB: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ErrUpstreamResponse
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(target); err != nil {
		return fmt.Errorf("decode TMDB response: %w", err)
	}
	return nil
}

func year(date string) int {
	if len(date) < 4 {
		return 0
	}
	parsed, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return parsed
}
