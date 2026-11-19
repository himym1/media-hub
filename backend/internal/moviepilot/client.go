package moviepilot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"media-hub/backend/internal/integration"
	"media-hub/backend/internal/search"
)

const (
	SourceID        = "moviepilot"
	SourceLabel     = "PT"
	defaultBaseURL  = "http://172.17.0.1:13001"
	searchBudget    = 45 * time.Second
	healthBudget    = 5 * time.Second
	downloadBudget  = 2 * time.Minute
	maxCandidates   = 20
	maxBodyBytes    = 4 << 20
	v3RefPrefix     = "mpv3:"
	v3CacheCapacity = 500
)

var (
	ErrNotConfigured    = errors.New("MoviePilot is not configured")
	ErrUnauthorized     = errors.New("MoviePilot rejected authentication")
	ErrUpstreamResponse = errors.New("MoviePilot returned an invalid response")
)

type protocolMode string

const (
	modeUnknown protocolMode = ""
	modeV2      protocolMode = "v2"
	modeV3      protocolMode = "v3"
)

type v3TorrentEntry struct {
	torrentIn   map[string]any
	mediaID     string
	mediaSource string
}

type Client struct {
	mutex     sync.RWMutex
	config    clientConfig
	client    *http.Client
	nextID    int64
	lastCheck integration.Health
	checkedAt time.Time
	mode      protocolMode
	v3Order   []string
	v3Cache   map[string]v3TorrentEntry
}

type clientConfig struct {
	baseURL string
	apiKey  string
}

type Media struct {
	TMDBID      string
	MediaID     string
	MediaSource string
	Title       string
	Year        int
	MediaType   string
}

type Torrent struct {
	Title     string
	SiteName  string
	Size      string
	Seeders   int
	MediaType string
	TMDBID    string
	Year      int
	Reference string
}

func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = searchBudget
	}
	client := &Client{client: &http.Client{Timeout: timeout}}
	client.Configure(baseURL, apiKey)
	return client
}

func (c *Client) Configure(baseURL, apiKey string) {
	c.mutex.Lock()
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	apiKey = strings.TrimSpace(apiKey)
	if baseURL == "" && apiKey != "" {
		baseURL = defaultBaseURL
	}
	c.config = clientConfig{baseURL: baseURL, apiKey: apiKey}
	c.lastCheck = integration.Health{}
	c.checkedAt = time.Time{}
	c.mode = modeUnknown
	c.v3Order = nil
	c.v3Cache = nil
	c.mutex.Unlock()
}

func (c *Client) Configured() bool {
	configuration := c.configuration()
	return configuration.baseURL != "" && configuration.apiKey != ""
}

func (c *Client) configuration() clientConfig {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.config
}

func (c *Client) Check(ctx context.Context) integration.Health {
	health := integration.Health{ID: SourceID, Label: "MoviePilot"}
	if !c.Configured() {
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未配置 MoviePilot"
		return health
	}
	c.mutex.RLock()
	if !c.checkedAt.IsZero() && time.Since(c.checkedAt) < 2*time.Minute {
		cached := c.lastCheck
		c.mutex.RUnlock()
		return cached
	}
	c.mutex.RUnlock()
	checkCtx, cancel := context.WithTimeout(ctx, healthBudget)
	defer cancel()
	raw, err := c.call(checkCtx, "tools/list", nil)
	detected := modeUnknown
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			health.Status = integration.StatusDegraded
			health.Detail = "服务可达，但 API Token 无效"
		} else {
			health.Status = integration.StatusUnavailable
			health.Detail = "无法读取 MoviePilot 状态"
		}
	} else {
		detected = detectProtocolMode(raw)
		health.Status = integration.StatusHealthy
		health.Detail = "PT 搜索与下载可用"
	}
	c.mutex.Lock()
	c.lastCheck = health
	c.checkedAt = time.Now()
	if detected != modeUnknown {
		c.mode = detected
	}
	c.mutex.Unlock()
	return health
}

func (c *Client) Search(ctx context.Context, query string) ([]search.Candidate, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	searchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), searchBudget)
	defer cancel()

	mode, err := c.ensureMode(searchCtx)
	if err != nil {
		return nil, err
	}

	var media Media
	var torrents []Torrent
	if mode == modeV3 {
		media, err = c.searchMediaV3(searchCtx, query)
		if err != nil {
			return nil, err
		}
		if media.TMDBID == "" && media.MediaID == "" {
			return nil, nil
		}
		torrents, err = c.searchTorrentsV3(searchCtx, media)
		if err != nil {
			return nil, err
		}
	} else {
		media, err = c.searchMedia(searchCtx, query)
		if err != nil {
			return nil, err
		}
		if media.TMDBID == "" {
			return nil, nil
		}
		if err := c.searchTorrents(searchCtx, media); err != nil {
			return nil, err
		}
		torrents, err = c.searchResults(searchCtx)
		if err != nil {
			return nil, err
		}
	}

	results := make([]search.Candidate, 0, len(torrents))
	for _, torrent := range torrents {
		if torrent.Reference == "" {
			continue
		}
		mediaType := torrent.MediaType
		if mediaType == "" {
			mediaType = media.MediaType
		}
		year := torrent.Year
		if year == 0 {
			year = media.Year
		}
		tmdbID := torrent.TMDBID
		if tmdbID == "" {
			tmdbID = media.TMDBID
		}
		results = append(results, search.Candidate{
			ID:            sanitizeID(torrent.Reference),
			Title:         firstNonEmpty(media.Title, torrent.Title),
			Year:          year,
			MediaType:     mediaType,
			TMDBID:        tmdbID,
			Provider:      torrent.SiteName,
			ReleaseTitle:  torrent.Title,
			SourceRef:     torrent.Reference,
			TransferState: "downloadable",
			Release:       releaseFacts(torrent.Title, torrent.Size),
		})
		if len(results) >= maxCandidates {
			break
		}
	}
	return results, nil
}

func (c *Client) StartDownload(ctx context.Context, request search.DownloadRequest) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	reference := strings.TrimSpace(request.Reference)
	if reference == "" {
		return search.Failure{Code: "source_invalid", Message: "下载引用无效", Retryable: false}
	}
	downloadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), downloadBudget)
	defer cancel()

	if strings.HasPrefix(reference, v3RefPrefix) {
		return c.startDownloadV3(downloadCtx, reference)
	}
	mode, err := c.ensureMode(downloadCtx)
	if err != nil {
		return publicFailure(err)
	}
	if mode == modeV3 {
		// 升级至 v3 后，旧版 v2 缓存引用已失效，提示重新搜索
		return search.Failure{Code: "source_stale", Message: "MoviePilot 搜索结果已过期，请重新搜索", Retryable: false}
	}

	payload, err := c.callTool(downloadCtx, "add_download_tasks", map[string]any{
		"torrent_url": []string{reference},
	})
	if err != nil {
		return publicFailure(err)
	}
	if lookupBool(payload, "success") {
		return nil
	}
	message := lookupString(jsonObject(payload), "message")
	if strings.Contains(message, "引用无效") || strings.Contains(strings.ToLower(message), "get_search_results") {
		return search.Failure{Code: "source_stale", Message: "MoviePilot 搜索结果已过期，请重新搜索", Retryable: false}
	}
	if message != "" && (strings.Contains(message, "失败") || strings.Contains(strings.ToLower(message), "fail")) {
		return search.Failure{Code: "source_unavailable", Message: "MoviePilot 未能加入下载队列", Retryable: true}
	}
	return nil
}

func (c *Client) startDownloadV3(ctx context.Context, reference string) error {
	entry, ok := c.lookupV3Torrent(reference)
	if !ok {
		// 内存缓存未命中（如服务重启后）时，回退查询最近一次搜索结果
		payload, err := c.callAPI(ctx, "search.results", nil, nil, nil)
		if err != nil {
			return publicFailure(err)
		}
		c.parseTorrentsV3(payload, Media{})
		entry, ok = c.lookupV3Torrent(reference)
		if !ok {
			return search.Failure{Code: "source_stale", Message: "MoviePilot 搜索结果已过期，请重新搜索", Retryable: false}
		}
	}

	body := map[string]any{
		"torrent_in": entry.torrentIn,
	}
	if entry.mediaID != "" {
		body["media_id"] = entry.mediaID
	}
	if entry.mediaSource != "" {
		body["media_source"] = entry.mediaSource
	}
	payload, err := c.callAPI(ctx, "download.add", nil, nil, body)
	if err != nil {
		return publicFailure(err)
	}
	if lookupBool(payload, "success") {
		return nil
	}
	message := lookupString(jsonObject(payload), "message")
	if strings.Contains(message, "签名") || strings.Contains(message, "过期") || strings.Contains(message, "引用无效") {
		return search.Failure{Code: "source_stale", Message: "MoviePilot 搜索结果已过期，请重新搜索", Retryable: false}
	}
	return search.Failure{Code: "source_unavailable", Message: "MoviePilot 未能加入下载队列", Retryable: true}
}

func (c *Client) ensureMode(ctx context.Context) (protocolMode, error) {
	c.mutex.RLock()
	mode := c.mode
	c.mutex.RUnlock()
	if mode != modeUnknown {
		return mode, nil
	}

	raw, err := c.call(ctx, "tools/list", nil)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return modeUnknown, err
		}
		// 兼容未实现 tools/list 的旧版或测试桩服务
		return modeV2, nil
	}
	detected := detectProtocolMode(raw)
	c.mutex.Lock()
	c.mode = detected
	c.mutex.Unlock()
	return detected, nil
}

func detectProtocolMode(raw json.RawMessage) protocolMode {
	var listed struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if json.Unmarshal(raw, &listed) != nil {
		return modeV2
	}
	hasGateway := false
	hasLegacy := false
	for _, tool := range listed.Tools {
		switch tool.Name {
		case "moviepilot_api":
			hasGateway = true
		case "search_torrents":
			hasLegacy = true
		}
	}
	if hasGateway && !hasLegacy {
		return modeV3
	}
	return modeV2
}

func (c *Client) searchMedia(ctx context.Context, query string) (Media, error) {
	title, year := splitTitleYear(query)
	mediaType := inferMediaType(query)
	tried := map[string]struct{}{mediaType: {}}
	for _, candidateType := range []string{mediaType, "movie", "tv"} {
		if _, seen := tried[candidateType]; seen && candidateType != mediaType {
			continue
		}
		tried[candidateType] = struct{}{}
		arguments := map[string]any{"title": title, "media_type": candidateType}
		if year > 0 {
			arguments["year"] = year
		}
		payload, err := c.callTool(ctx, "search_media", arguments)
		if err != nil {
			return Media{}, err
		}
		if media, ok := pickMedia(payload, candidateType, year); ok {
			return media, nil
		}
	}
	return Media{}, nil
}

func (c *Client) searchMediaV3(ctx context.Context, query string) (Media, error) {
	title, year := splitTitleYear(query)
	hintType := inferMediaType(query)
	payload, err := c.callAPI(ctx, "media.search", nil, map[string]any{
		"title":        title,
		"media_source": []string{"themoviedb"},
		"page":         1,
		"count":        8,
	}, nil)
	if err != nil {
		return Media{}, err
	}
	if _, failed := hasExplicitFailure(payload); failed {
		return Media{}, ErrUpstreamResponse
	}
	media, _ := pickMedia(payload, hintType, year)
	return media, nil
}

func (c *Client) searchTorrents(ctx context.Context, media Media) error {
	arguments := map[string]any{"tmdb_id": parseTMDBID(media.TMDBID), "media_type": mcpMediaType(media.MediaType)}
	_, err := c.callTool(ctx, "search_torrents", arguments)
	return err
}

func (c *Client) searchTorrentsV3(ctx context.Context, media Media) ([]Torrent, error) {
	mediaID := firstNonEmpty(media.MediaID, media.TMDBID)
	mediaSource := firstNonEmpty(media.MediaSource, "themoviedb")
	payload, err := c.callAPI(
		ctx,
		"search.torrents",
		map[string]any{"media_id": mediaID},
		map[string]any{
			"media_source": mediaSource,
			"mtype":        mcpMediaType(media.MediaType),
		},
		nil,
	)
	if err != nil {
		return nil, err
	}
	if _, failed := hasExplicitFailure(payload); failed {
		return nil, ErrUpstreamResponse
	}
	torrents := c.parseTorrentsV3(payload, media)
	if len(torrents) == 0 && lookupCollectionTotal(payload) > 0 {
		resultsPayload, err := c.callAPI(ctx, "search.results", nil, nil, nil)
		if err != nil {
			return nil, err
		}
		if _, failed := hasExplicitFailure(resultsPayload); failed {
			return nil, ErrUpstreamResponse
		}
		torrents = c.parseTorrentsV3(resultsPayload, media)
	}
	return torrents, nil
}

func (c *Client) searchResults(ctx context.Context) ([]Torrent, error) {
	payload, err := c.callTool(ctx, "get_search_results", map[string]any{
		"page":                1,
		"include_description": false,
	})
	if err != nil {
		return nil, err
	}
	return parseTorrents(payload), nil
}

func (c *Client) callAPI(ctx context.Context, operationID string, pathParams, query map[string]any, body any) (json.RawMessage, error) {
	arguments := map[string]any{
		"operation_id": operationID,
	}
	if len(pathParams) > 0 {
		arguments["path_params"] = pathParams
	}
	if len(query) > 0 {
		arguments["query"] = query
	}
	if body != nil {
		arguments["body"] = body
	}
	return c.callTool(ctx, "moviepilot_api", arguments)
}

func (c *Client) callTool(ctx context.Context, name string, arguments map[string]any) (json.RawMessage, error) {
	return c.call(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": arguments,
	})
}

func (c *Client) call(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	configuration := c.configuration()
	if configuration.baseURL == "" || configuration.apiKey == "" {
		return nil, ErrNotConfigured
	}
	c.mutex.Lock()
	c.nextID++
	id := c.nextID
	c.mutex.Unlock()
	envelope := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		envelope["params"] = params
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	endpoint, err := url.JoinPath(configuration.baseURL, "/api/v1/mcp")
	if err != nil {
		return nil, ErrUpstreamResponse
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-API-KEY", configuration.apiKey)
	request.Header.Set("User-Agent", "Media-Hub/moviepilot")
	response, err := c.httpClient(ctx).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxBodyBytes {
		return nil, ErrUpstreamResponse
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, ErrUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, ErrUpstreamResponse
	}
	var rpc struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &rpc); err != nil {
		return nil, ErrUpstreamResponse
	}
	if rpc.Error != nil {
		return nil, ErrUpstreamResponse
	}
	if method == "tools/list" {
		return rpc.Result, nil
	}
	return decodeToolResult(rpc.Result)
}

func (c *Client) httpClient(ctx context.Context) *http.Client {
	if deadline, ok := ctx.Deadline(); ok {
		remain := time.Until(deadline)
		if remain > 0 && remain > c.client.Timeout {
			return &http.Client{Timeout: remain, Transport: c.client.Transport}
		}
	}
	return c.client
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func decodeToolResult(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return json.RawMessage(`{}`), nil
	}
	var result struct {
		IsError           bool            `json:"isError"`
		StructuredContent json.RawMessage `json:"structuredContent"`
		Content           []toolContent   `json:"content"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		if json.Valid(raw) {
			return raw, nil
		}
		return nil, ErrUpstreamResponse
	}
	if result.IsError {
		if message := firstToolText(result.Content); message != "" {
			if obj := jsonObject([]byte(message)); obj != nil {
				if _, ok := obj["success"]; ok {
					return json.RawMessage(message), nil
				}
			}
			return marshalToolMessage(false, message)
		}
		return nil, ErrUpstreamResponse
	}
	if len(bytes.TrimSpace(result.StructuredContent)) > 0 {
		return result.StructuredContent, nil
	}
	if text := firstToolText(result.Content); text != "" {
		if json.Valid([]byte(text)) {
			return json.RawMessage(text), nil
		}
		ok := !strings.Contains(text, "失败") && !strings.Contains(strings.ToLower(text), "fail")
		return marshalToolMessage(ok, text)
	}
	if json.Valid(raw) {
		return raw, nil
	}
	return json.RawMessage(`{}`), nil
}

func firstToolText(items []toolContent) string {
	for _, item := range items {
		if !strings.EqualFold(item.Type, "text") {
			continue
		}
		if text := strings.TrimSpace(item.Text); text != "" {
			return text
		}
	}
	return ""
}

func marshalToolMessage(success bool, message string) (json.RawMessage, error) {
	raw, err := json.Marshal(map[string]any{"success": success, "message": message})
	if err != nil {
		return nil, ErrUpstreamResponse
	}
	return raw, nil
}

func hasExplicitFailure(payload json.RawMessage) (string, bool) {
	obj := jsonObject(payload)
	if obj == nil {
		return "", false
	}
	val, ok := obj["success"]
	if !ok {
		return "", false
	}
	success, isBool := val.(bool)
	if isBool && !success {
		return lookupString(obj, "message"), true
	}
	return "", false
}

func lookupCollectionTotal(payload json.RawMessage) int {
	obj := jsonObject(payload)
	if obj == nil {
		return 0
	}
	collection := nestedObject(obj, "collection")
	if collection == nil {
		return 0
	}
	return lookupInt(collection, "total_count")
}

func firstMedia(payload json.RawMessage, fallbackType string) (Media, bool) {
	return pickMedia(payload, fallbackType, 0)
}

func pickMedia(payload json.RawMessage, fallbackType string, year int) (Media, bool) {
	items := jsonArray(payload)
	if len(items) == 0 {
		if item := jsonObject(payload); item != nil {
			if raw, err := json.Marshal(item); err == nil {
				items = []json.RawMessage{raw}
			}
		}
	}
	var fallback Media
	hasFallback := false
	for _, item := range items {
		var row map[string]any
		if json.Unmarshal(item, &row) != nil {
			continue
		}
		tmdbID := firstNonEmpty(lookupString(row, "tmdb_id"), lookupString(row, "tmdbid"), lookupString(row, "id"))
		mediaID := firstNonEmpty(lookupString(row, "media_id"), tmdbID)
		if (tmdbID == "" || tmdbID == "0") && (mediaID == "" || mediaID == "0") {
			continue
		}
		media := Media{
			TMDBID:      tmdbID,
			MediaID:     mediaID,
			MediaSource: firstNonEmpty(lookupString(row, "media_source"), "themoviedb"),
			Title:       firstNonEmpty(lookupString(row, "title"), lookupString(row, "name")),
			Year:        lookupInt(row, "year"),
			MediaType:   normalizeMediaType(firstNonEmpty(lookupString(row, "media_type"), lookupString(row, "type"), fallbackType)),
		}
		if year > 0 && media.Year == year {
			return media, true
		}
		if hasFallback {
			continue
		}
		if year == 0 || media.Year == 0 {
			fallback = media
			hasFallback = true
		}
	}
	if year > 0 {
		return Media{}, false
	}
	return fallback, hasFallback
}

func extractTorrentRows(payload json.RawMessage) []json.RawMessage {
	rows := jsonArray(payload)
	if len(rows) > 0 {
		return rows
	}
	root := jsonObject(payload)
	if root == nil {
		return nil
	}
	if nested, ok := root["results"]; ok {
		if raw, err := json.Marshal(nested); err == nil {
			if parsed := jsonArray(raw); len(parsed) > 0 {
				return parsed
			}
		}
	}
	if dataObj := nestedObject(root, "data"); dataObj != nil {
		if nested, ok := dataObj["results"]; ok {
			if raw, err := json.Marshal(nested); err == nil {
				return jsonArray(raw)
			}
		}
	}
	return nil
}

func parseTorrents(payload json.RawMessage) []Torrent {
	rows := extractTorrentRows(payload)
	results := make([]Torrent, 0, len(rows))
	for _, row := range rows {
		var item map[string]any
		if json.Unmarshal(row, &item) != nil {
			continue
		}
		info := nestedObject(item, "torrent_info")
		media := nestedObject(item, "media_info")
		if info == nil {
			info = item
		}
		reference := firstNonEmpty(lookupString(info, "torrent_url"), lookupString(info, "enclosure"))
		if reference == "" || looksLikeURL(reference) {
			continue
		}
		results = append(results, Torrent{
			Title:     lookupString(info, "title"),
			SiteName:  lookupString(info, "site_name"),
			Size:      lookupString(info, "size"),
			Seeders:   lookupInt(info, "seeders"),
			MediaType: normalizeMediaType(firstNonEmpty(lookupString(media, "media_type"), lookupString(media, "type"))),
			TMDBID:    firstNonEmpty(lookupString(media, "tmdb_id"), lookupString(media, "tmdbid")),
			Year:      lookupInt(media, "year"),
			Reference: reference,
		})
	}
	return results
}

func (c *Client) parseTorrentsV3(payload json.RawMessage, fallbackMedia Media) []Torrent {
	rows := extractTorrentRows(payload)
	results := make([]Torrent, 0, len(rows))
	for _, row := range rows {
		var item map[string]any
		if json.Unmarshal(row, &item) != nil {
			continue
		}
		info := nestedObject(item, "torrent_info")
		media := nestedObject(item, "media_info")
		if info == nil {
			info = item
		}
		if firstNonEmpty(lookupString(info, "enclosure"), lookupString(info, "torrent_url")) == "" {
			continue
		}
		reference := v3TorrentRef(info)
		if reference == "" {
			continue
		}
		mediaID := firstNonEmpty(
			lookupString(media, "media_id"),
			lookupString(media, "tmdb_id"),
			lookupString(media, "tmdbid"),
			lookupString(info, "media_id"),
			fallbackMedia.MediaID,
			fallbackMedia.TMDBID,
		)
		mediaSource := firstNonEmpty(
			lookupString(media, "media_source"),
			lookupString(info, "media_source"),
			fallbackMedia.MediaSource,
		)
		c.cacheV3Torrent(reference, v3TorrentEntry{
			torrentIn:   info,
			mediaID:     mediaID,
			mediaSource: mediaSource,
		})
		results = append(results, Torrent{
			Title:     lookupString(info, "title"),
			SiteName:  lookupString(info, "site_name"),
			Size:      lookupString(info, "size"),
			Seeders:   lookupInt(info, "seeders"),
			MediaType: normalizeMediaType(firstNonEmpty(lookupString(media, "media_type"), lookupString(media, "type"))),
			TMDBID:    firstNonEmpty(lookupString(media, "tmdb_id"), lookupString(media, "tmdbid")),
			Year:      lookupInt(media, "year"),
			Reference: reference,
		})
	}
	return results
}

func v3TorrentRef(info map[string]any) string {
	enclosure := firstNonEmpty(lookupString(info, "enclosure"), lookupString(info, "torrent_url"))
	if enclosure == "" {
		return ""
	}
	// 使用单向摘要生成不透明引用，避免在客户端或持久层暴露站点 Cookie 与带签名种子链接
	sum := sha256.Sum256([]byte(strings.Join([]string{
		lookupString(info, "site"),
		lookupString(info, "site_name"),
		lookupString(info, "title"),
		enclosure,
		lookupString(info, "page_url"),
		lookupString(info, "size"),
	}, "\x00")))
	return v3RefPrefix + hex.EncodeToString(sum[:8])
}

func (c *Client) cacheV3Torrent(reference string, entry v3TorrentEntry) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.v3Cache == nil {
		c.v3Cache = make(map[string]v3TorrentEntry)
	}
	if _, exists := c.v3Cache[reference]; !exists {
		if len(c.v3Order) >= v3CacheCapacity {
			oldest := c.v3Order[0]
			c.v3Order = c.v3Order[1:]
			delete(c.v3Cache, oldest)
		}
		c.v3Order = append(c.v3Order, reference)
	}
	c.v3Cache[reference] = entry
}

func (c *Client) lookupV3Torrent(reference string) (v3TorrentEntry, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	if c.v3Cache == nil {
		return v3TorrentEntry{}, false
	}
	entry, ok := c.v3Cache[reference]
	return entry, ok
}

func jsonArray(raw json.RawMessage) []json.RawMessage {
	var rows []json.RawMessage
	if json.Unmarshal(raw, &rows) == nil {
		return rows
	}
	var wrapped struct {
		Result json.RawMessage `json:"result"`
		Data   json.RawMessage `json:"data"`
		Items  json.RawMessage `json:"items"`
	}
	if json.Unmarshal(raw, &wrapped) != nil {
		return nil
	}
	for _, candidate := range []json.RawMessage{wrapped.Result, wrapped.Data, wrapped.Items} {
		if json.Unmarshal(candidate, &rows) == nil {
			return rows
		}
	}
	return nil
}

func jsonObject(raw json.RawMessage) map[string]any {
	var object map[string]any
	if json.Unmarshal(raw, &object) != nil {
		return nil
	}
	return object
}

func nestedObject(root map[string]any, key string) map[string]any {
	value, ok := root[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case map[string]any:
		return typed
	default:
		raw, err := json.Marshal(value)
		if err != nil {
			return nil
		}
		return jsonObject(raw)
	}
}

func lookupString(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return strings.TrimSpace(typed.String())
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case int:
		return strconv.Itoa(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func lookupInt(values map[string]any, key string) int {
	if values == nil {
		return 0
	}
	value, ok := values[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	default:
		return 0
	}
}

func lookupBool(raw json.RawMessage, key string) bool {
	object := jsonObject(raw)
	if object == nil {
		return false
	}
	value, ok := object[key]
	if !ok {
		return false
	}
	flag, _ := value.(bool)
	return flag
}

func parseTMDBID(value string) any {
	if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && parsed > 0 {
		return parsed
	}
	return strings.TrimSpace(value)
}

func mcpMediaType(value string) string {
	if normalizeMediaType(value) == "series" {
		return "tv"
	}
	return "movie"
}

func normalizeMediaType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "tv", "series", "show", "season", "电视剧", "剧集", "连续剧":
		return "series"
	default:
		return "movie"
	}
}

func inferMediaType(query string) string {
	lower := strings.ToLower(query)
	if strings.Contains(query, "第") && (strings.Contains(query, "季") || strings.Contains(query, "集")) {
		return "tv"
	}
	if strings.Contains(lower, "s0") || strings.Contains(lower, "season") || strings.Contains(lower, "episode") {
		return "tv"
	}
	return "movie"
}

func splitTitleYear(query string) (string, int) {
	query = strings.TrimSpace(query)
	parts := strings.Fields(query)
	if len(parts) == 0 {
		return query, 0
	}
	last := parts[len(parts)-1]
	if len(last) == 4 {
		if year, err := strconv.Atoi(last); err == nil && year >= 1900 && year <= 2100 {
			return strings.TrimSpace(strings.TrimSuffix(query, last)), year
		}
	}
	return query, 0
}

func sanitizeID(reference string) string {
	replacer := strings.NewReplacer(":", "-", "/", "-", " ", "")
	id := replacer.Replace(strings.TrimSpace(reference))
	if id == "" {
		return "pt"
	}
	return id
}

func looksLikeURL(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "magnet:")
}

func publicFailure(err error) error {
	switch {
	case errors.Is(err, ErrUnauthorized):
		return search.Failure{Code: "source_unauthorized", Message: "MoviePilot 认证失败", Retryable: true}
	case errors.Is(err, ErrNotConfigured):
		return search.Failure{Code: "source_unconfigured", Message: "MoviePilot 未配置", Retryable: false}
	case isTimeout(err):
		return search.Failure{Code: "source_unavailable", Message: "MoviePilot 提交下载超时", Retryable: true}
	default:
		return search.Failure{Code: "source_unavailable", Message: "MoviePilot 暂时不可用", Retryable: true}
	}
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
