package moviepilot

import (
	"bytes"
	"context"
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
	SourceID       = "moviepilot"
	SourceLabel    = "PT"
	defaultBaseURL = "http://172.17.0.1:13001"
	searchBudget   = 45 * time.Second
	healthBudget   = 5 * time.Second
	downloadBudget = 2 * time.Minute
	maxCandidates  = 20
	maxBodyBytes   = 4 << 20
)

var (
	ErrNotConfigured    = errors.New("MoviePilot is not configured")
	ErrUnauthorized     = errors.New("MoviePilot rejected authentication")
	ErrUpstreamResponse = errors.New("MoviePilot returned an invalid response")
)

type Client struct {
	mutex     sync.RWMutex
	config    clientConfig
	client    *http.Client
	nextID    int64
	lastCheck integration.Health
	checkedAt time.Time
}

type clientConfig struct {
	baseURL string
	apiKey  string
}

type Media struct {
	TMDBID    string
	Title     string
	Year      int
	MediaType string
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
	_, err := c.call(checkCtx, "tools/list", nil)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			health.Status = integration.StatusDegraded
			health.Detail = "服务可达，但 API Token 无效"
		} else {
			health.Status = integration.StatusUnavailable
			health.Detail = "无法读取 MoviePilot 状态"
		}
	} else {
		health.Status = integration.StatusHealthy
		health.Detail = "PT 搜索与下载可用"
	}
	c.mutex.Lock()
	c.lastCheck = health
	c.checkedAt = time.Now()
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
	media, err := c.searchMedia(searchCtx, query)
	if err != nil {
		return nil, err
	}
	if media.TMDBID == "" {
		return nil, nil
	}
	if err := c.searchTorrents(searchCtx, media); err != nil {
		return nil, err
	}
	torrents, err := c.searchResults(searchCtx)
	if err != nil {
		return nil, err
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

func (c *Client) searchTorrents(ctx context.Context, media Media) error {
	arguments := map[string]any{"tmdb_id": parseTMDBID(media.TMDBID), "media_type": mcpMediaType(media.MediaType)}
	_, err := c.callTool(ctx, "search_torrents", arguments)
	return err
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
		if tmdbID == "" || tmdbID == "0" {
			continue
		}
		media := Media{
			TMDBID:    tmdbID,
			Title:     firstNonEmpty(lookupString(row, "title"), lookupString(row, "name")),
			Year:      lookupInt(row, "year"),
			MediaType: normalizeMediaType(firstNonEmpty(lookupString(row, "media_type"), lookupString(row, "type"), fallbackType)),
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

func parseTorrents(payload json.RawMessage) []Torrent {
	root := jsonObject(payload)
	rows := jsonArray(payload)
	if root != nil {
		if nested, ok := root["results"]; ok {
			if raw, err := json.Marshal(nested); err == nil {
				rows = jsonArray(raw)
			}
		}
	}
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
	case "tv", "series", "show", "season":
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
