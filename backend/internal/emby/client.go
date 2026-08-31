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
	"unicode"

	"media-hub/backend/internal/integration"
)

var (
	ErrNotConfigured    = errors.New("Emby is not configured")
	ErrMissingAPIKey    = errors.New("Emby API key is missing")
	ErrUnauthorized     = errors.New("Emby rejected authentication")
	ErrItemNotFound     = errors.New("Emby item was not found")
	ErrDeleteRejected   = errors.New("Emby rejected the delete")
	ErrDeleteNeedsUser  = errors.New("Emby delete requires a user password")
	ErrUpstreamResponse = errors.New("Emby returned an invalid response")
)

type RuntimeConfig struct {
	BaseURL         string
	APIKey          string
	UserID          string
	Password        string
	PlaybackBaseURL string
	MovieLibraryID  string
	SeriesLibraryID string
}

type clientConfig struct {
	baseURL         string
	apiKey          string
	userID          string
	password        string
	playbackBaseURL string
	movieLibraryID  string
	seriesLibraryID string
}

type Client struct {
	mutex          sync.RWMutex
	config         clientConfig
	client         *http.Client
	imageClient    *http.Client
	subtitleClient *http.Client
	sessionToken   string
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
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Type               string            `json:"type"`
	Year               int               `json:"year,omitempty"`
	ProviderIDs        map[string]string `json:"providerIds,omitempty"`
	Season             int               `json:"season,omitempty"`
	Episode            int               `json:"episode,omitempty"`
	PlaybackPositionMS int64             `json:"playbackPositionMs,omitempty"`
	Played             bool              `json:"played,omitempty"`
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
	Path              string            `json:"Path"`
	SeriesID          string            `json:"SeriesId"`
	SeriesName        string            `json:"SeriesName"`
	UserData          userData          `json:"UserData"`
}

type SubtitleTarget struct {
	ID            string
	Type          string
	Name          string
	SeriesName    string
	OriginalTitle string
	Year          int
	Season        int
	Episode       int
	Path          string
}

type userData struct {
	PlaybackPositionTicks int64 `json:"PlaybackPositionTicks"`
	Played                bool  `json:"Played"`
}

type mediaSource struct {
	ID                 string `json:"Id"`
	Path               string `json:"Path"`
	DirectStreamURL    string `json:"DirectStreamUrl"`
	SupportsDirectPlay bool   `json:"SupportsDirectPlay"`
	Container          string `json:"Container"`
}

type playbackInfoResponse struct {
	MediaSources  []mediaSource `json:"MediaSources"`
	PlaySessionID string        `json:"PlaySessionId"`
}

func NewClient(baseURL, apiKey string, timeout time.Duration, userID ...string) *Client {
	configuredUserID := ""
	if len(userID) > 0 {
		configuredUserID = userID[0]
	}
	return NewConfiguredClient(RuntimeConfig{
		BaseURL: baseURL, APIKey: apiKey, UserID: configuredUserID,
	}, timeout)
}

func NewConfiguredClient(configuration RuntimeConfig, timeout time.Duration) *Client {
	imageTimeout := timeout
	if imageTimeout < 45*time.Second {
		imageTimeout = 45 * time.Second
	}
	subtitleTimeout := timeout
	if subtitleTimeout < defaultSubtitleTimeout {
		subtitleTimeout = defaultSubtitleTimeout
	}
	redirectPolicy := func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &Client{
		config: runtimeClientConfig(configuration),
		client: &http.Client{
			Timeout:       timeout,
			CheckRedirect: redirectPolicy,
		},
		imageClient: &http.Client{
			Timeout:       imageTimeout,
			CheckRedirect: redirectPolicy,
		},
		subtitleClient: &http.Client{
			Timeout:       subtitleTimeout,
			CheckRedirect: redirectPolicy,
		},
	}
}

func (c *Client) Configure(configuration RuntimeConfig) {
	c.mutex.Lock()
	c.config = runtimeClientConfig(configuration)
	c.sessionToken = ""
	c.mutex.Unlock()
}

func runtimeClientConfig(configuration RuntimeConfig) clientConfig {
	return clientConfig{
		baseURL:         strings.TrimRight(strings.TrimSpace(configuration.BaseURL), "/"),
		apiKey:          strings.TrimSpace(configuration.APIKey),
		userID:          strings.TrimSpace(configuration.UserID),
		password:        strings.TrimSpace(configuration.Password),
		playbackBaseURL: strings.TrimRight(strings.TrimSpace(configuration.PlaybackBaseURL), "/"),
		movieLibraryID:  strings.TrimSpace(configuration.MovieLibraryID),
		seriesLibraryID: strings.TrimSpace(configuration.SeriesLibraryID),
	}
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

func (c *Client) Libraries(context.Context) ([]Library, error) {
	configuration := c.configuration()
	if err := validateAuthenticated(configuration); err != nil {
		return nil, err
	}
	libraries := make([]Library, 0, 2)
	if configuration.movieLibraryID != "" {
		libraries = append(libraries, Library{
			ID: configuration.movieLibraryID, Name: "115电影", CollectionType: "movies",
		})
	}
	if configuration.seriesLibraryID != "" && configuration.seriesLibraryID != configuration.movieLibraryID {
		libraries = append(libraries, Library{
			ID: configuration.seriesLibraryID, Name: "115电视剧", CollectionType: "tvshows",
		})
	}
	return libraries, nil
}

func configuredLibraryIDs(configuration clientConfig) []string {
	ids := make([]string, 0, 2)
	if configuration.movieLibraryID != "" {
		ids = append(ids, configuration.movieLibraryID)
	}
	if configuration.seriesLibraryID != "" && configuration.seriesLibraryID != configuration.movieLibraryID {
		ids = append(ids, configuration.seriesLibraryID)
	}
	return ids
}

func libraryAllowed(configuration clientConfig, libraryID string) bool {
	for _, configuredID := range configuredLibraryIDs(configuration) {
		if libraryID == configuredID {
			return true
		}
	}
	return false
}

func (c *Client) SearchItems(ctx context.Context, queryText string, limit int) (SearchResult, error) {
	configuration := c.configuration()
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
		"Fields":           {"ProviderIds,MediaSources,Path"},
		"IncludeItemTypes": {"Movie,Series"},
		"Limit":            {"100"},
		"Recursive":        {"true"},
		"SearchTerm":       {queryText},
	}
	if configuration.userID != "" {
		query.Set("UserId", configuration.userID)
	}
	var response itemResponse
	seen := make(map[string]struct{})
	for _, libraryID := range configuredLibraryIDs(configuration) {
		query.Set("ParentId", libraryID)
		var libraryResponse itemResponse
		if err := c.getJSON(ctx, configuration, "Items", query, true, &libraryResponse); err != nil {
			return SearchResult{}, err
		}
		for _, item := range libraryResponse.Items {
			if _, exists := seen[item.ID]; item.ID != "" && exists {
				continue
			}
			if item.ID != "" {
				seen[item.ID] = struct{}{}
			}
			response.Items = append(response.Items, item)
		}
	}
	seriesIDs := make(map[string]struct{})
	for _, item := range response.Items {
		if item.Type != "Series" || item.ID == "" {
			continue
		}
		visible, err := c.cloudItemVisible(ctx, configuration, item)
		if err != nil {
			return SearchResult{}, err
		}
		if visible {
			seriesIDs[item.ID] = struct{}{}
		}
	}
	filtered := filter115Items(response.Items, seriesIDs)
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return publicItems(itemResponse{Items: filtered, TotalRecordCount: len(filtered)}), nil
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
	if !libraryAllowed(configuration, libraryID) {
		return SearchResult{}, ErrItemNotFound
	}
	if offset < 0 {
		offset = 0
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	query := url.Values{
		"Fields":           {"ProviderIds,UserData,MediaSources,Path"},
		"IncludeItemTypes": {"Movie,Series"},
		"Limit":            {"10000"},
		"ParentId":         {libraryID},
		"Recursive":        {"true"},
		"SortBy":           {"SortName"},
		"SortOrder":        {"Ascending"},
		"StartIndex":       {"0"},
	}
	if configuration.userID != "" {
		query.Set("UserId", configuration.userID)
	}
	var response itemResponse
	if err := c.getJSON(ctx, configuration, "Items", query, true, &response); err != nil {
		return SearchResult{}, err
	}
	seriesIDs, err := c.cloudSeriesIDs(ctx, configuration, libraryID)
	if err != nil {
		return SearchResult{}, err
	}
	filtered := filter115Items(response.Items, seriesIDs)
	if offset > len(filtered) {
		offset = len(filtered)
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return publicItems(itemResponse{Items: filtered[offset:end], TotalRecordCount: len(filtered)}), nil
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
		"Fields": {"CommunityRating,Genres,MediaSources,OriginalTitle,Overview,Path,ProviderIds,RunTimeTicks,SeriesName,UserData"},
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
	visible, err := c.cloudItemVisible(ctx, configuration, item)
	if err != nil {
		return ItemDetail{}, err
	}
	if !visible {
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

func (c *Client) SubtitleTarget(ctx context.Context, itemID string) (SubtitleTarget, error) {
	configuration := c.configuration()
	if err := validateAuthenticated(configuration); err != nil {
		return SubtitleTarget{}, err
	}
	itemID = strings.TrimSpace(itemID)
	if itemID == "" || !validEmbyIdentifier(itemID) {
		return SubtitleTarget{}, ErrUpstreamResponse
	}
	query := url.Values{"Fields": {"MediaSources,OriginalTitle,ParentIndexNumber,IndexNumber,Path,ProductionYear,SeriesName"}}
	endpointPath := path.Join("Items", itemID)
	if configuration.userID != "" {
		endpointPath = path.Join("Users", configuration.userID, "Items", itemID)
	}
	var item baseItem
	if err := c.getJSONWithNotFound(ctx, configuration, endpointPath, query, true, &item); err != nil {
		return SubtitleTarget{}, err
	}
	if !strings.EqualFold(item.ID, itemID) {
		return SubtitleTarget{}, ErrItemNotFound
	}
	mediaPath := strings.TrimSpace(item.Path)
	if mediaPath == "" && len(item.MediaSources) > 0 {
		mediaPath = strings.TrimSpace(item.MediaSources[0].Path)
	}
	return SubtitleTarget{
		ID:            item.ID,
		Type:          item.Type,
		Name:          item.Name,
		SeriesName:    item.SeriesName,
		OriginalTitle: item.OriginalTitle,
		Year:          item.ProductionYear,
		Season:        item.ParentIndexNumber,
		Episode:       item.IndexNumber,
		Path:          mediaPath,
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
			"Fields":              {"ProviderIds,OriginalTitle,Path"},
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
				return publicItem(item), true, nil
			}
		}
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return Item{}, false, nil
	}
	if err := validateAuthenticated(configuration); err != nil {
		return Item{}, false, err
	}
	query := url.Values{
		"Fields":           {"ProviderIds,OriginalTitle,Path"},
		"IncludeItemTypes": {expectedType},
		"Limit":            {"50"},
		"Recursive":        {"true"},
		"SearchTerm":       {title},
	}
	if configuration.userID != "" {
		query.Set("UserId", configuration.userID)
	}
	var response itemResponse
	if err := c.getJSON(ctx, configuration, "Items", query, true, &response); err != nil {
		return Item{}, false, err
	}
	var best baseItem
	bestScore := 0
	for _, item := range response.Items {
		if item.ID == "" || item.Type != expectedType {
			continue
		}
		score, ok := indexedTitleMatchScore(title, year, item)
		if !ok || score < bestScore {
			continue
		}
		best = item
		bestScore = score
	}
	if bestScore == 0 {
		return Item{}, false, nil
	}
	return publicItem(best), true, nil
}

// indexedTitleMatchScore ranks Emby candidates for workflow completion.
// Exact Name/OriginalTitle beats release-style names that merely contain the title.
// Fuzzy matches require a year agreement when both sides provide one, and reject
// longer titles that only share a prefix (e.g. 超凡蜘蛛侠 must not match 超凡蜘蛛侠2).
func indexedTitleMatchScore(title string, year int, item baseItem) (int, bool) {
	title = strings.TrimSpace(title)
	if title == "" {
		return 0, false
	}
	name := strings.TrimSpace(item.Name)
	original := strings.TrimSpace(item.OriginalTitle)
	pathValue := strings.TrimSpace(item.Path)
	if year > 0 && item.ProductionYear > 0 && item.ProductionYear != year {
		return 0, false
	}
	if strings.EqualFold(name, title) || strings.EqualFold(original, title) {
		return 3, true
	}
	if containsTitleToken(name, title) || containsTitleToken(original, title) || containsTitleToken(pathValue, title) {
		if year > 0 && item.ProductionYear <= 0 {
			return 1, true
		}
		return 2, true
	}
	return 0, false
}

func containsTitleToken(haystack, title string) bool {
	haystack = strings.TrimSpace(haystack)
	title = strings.TrimSpace(title)
	if haystack == "" || title == "" {
		return false
	}
	haystackFold := strings.ToLower(haystack)
	titleFold := strings.ToLower(title)
	haystackRunes := []rune(haystackFold)
	titleRunes := []rune(titleFold)
	if len(titleRunes) == 0 || len(titleRunes) > len(haystackRunes) {
		return false
	}
	for index := 0; index+len(titleRunes) <= len(haystackRunes); index++ {
		if string(haystackRunes[index:index+len(titleRunes)]) != titleFold {
			continue
		}
		end := index + len(titleRunes)
		if end == len(haystackRunes) || !isTitleContinuation(haystackRunes[end]) {
			return true
		}
	}
	return false
}

func isTitleContinuation(value rune) bool {
	return unicode.IsLetter(value) || unicode.IsDigit(value)
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

// ApplyTMDBMetadata identifies an Emby item with a known TMDB id and refreshes
// metadata/images. Prefer this over library refresh alone when folder names are
// non-standard and ProviderIds are empty.
func (c *Client) ApplyTMDBMetadata(ctx context.Context, itemID, title string, year int, tmdbID string, replaceAllImages bool) error {
	configuration := c.configuration()
	if err := validateAuthenticated(configuration); err != nil {
		return err
	}
	itemID = strings.TrimSpace(itemID)
	tmdbID = strings.TrimSpace(tmdbID)
	if itemID == "" || tmdbID == "" {
		return ErrUpstreamResponse
	}
	payload := map[string]any{
		"ProviderIds": map[string]string{"Tmdb": tmdbID},
	}
	if name := strings.TrimSpace(title); name != "" {
		payload["Name"] = name
	}
	if year > 0 {
		payload["ProductionYear"] = year
	}
	query := url.Values{}
	if replaceAllImages {
		query.Set("ReplaceAllImages", "true")
	}
	return c.postJSONBody(ctx, configuration, path.Join("Items", "RemoteSearch", "Apply", itemID), query, payload, nil, false)
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
		PlaybackPositionMS: max(0, item.UserData.PlaybackPositionTicks/10_000),
		Played:             item.UserData.Played,
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
		applyEmbyAuth(request, configuration)
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
	return c.postJSONBody(ctx, configuration, endpointPath, query, map[string]any{}, target, false)
}

func (c *Client) postJSONBody(
	ctx context.Context,
	configuration clientConfig,
	endpointPath string,
	query url.Values,
	body any,
	target any,
	mapNotFound bool,
) error {
	endpoint, err := endpointURL(configuration.baseURL, endpointPath, query)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode Emby request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create Emby request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Media-Hub/emby")
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
	return decodeEmbyBody(response, target)
}

func applyEmbyAuth(request *http.Request, configuration clientConfig) {
	request.Header.Set("X-Emby-Token", configuration.apiKey)
	authorization := embyAuthorization(configuration)
	request.Header.Set("X-Emby-Authorization", authorization)
	request.Header.Set("Authorization", authorization)
	if configuration.userID != "" {
		request.Header.Set("X-Emby-UserId", configuration.userID)
	}
}

func applyEmbyClientAuth(request *http.Request) {
	authorization := strings.Join([]string{
		`MediaBrowser Client="Media Hub"`,
		`Device="MediaHub"`,
		`DeviceId="media-hub"`,
		`Version="1.0"`,
	}, ", ")
	request.Header.Set("X-Emby-Authorization", authorization)
	request.Header.Set("Authorization", authorization)
}

func embyAuthorization(configuration clientConfig) string {
	parts := []string{
		`MediaBrowser Client="Media Hub"`,
		`Device="MediaHub"`,
		`DeviceId="media-hub"`,
		`Version="1.0"`,
	}
	if configuration.userID != "" {
		parts = append(parts, `UserId="`+configuration.userID+`"`)
	}
	if configuration.apiKey != "" {
		parts = append(parts, `Token="`+configuration.apiKey+`"`)
	}
	return strings.Join(parts, ", ")
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
