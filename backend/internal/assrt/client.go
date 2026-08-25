package assrt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"media-hub/backend/internal/integration"
)

const defaultBaseURL = "https://api.assrt.net"

var (
	ErrNotConfigured    = errors.New("Assrt is not configured")
	ErrUnauthorized     = errors.New("Assrt rejected authentication")
	ErrUpstreamResponse = errors.New("Assrt returned an invalid response")
	ErrUnsupportedFile  = errors.New("Assrt subtitle archive is not supported")
)

type Client struct {
	mutex     sync.RWMutex
	config    clientConfig
	client    *http.Client
	lastCheck integration.Health
	checkedAt time.Time
}

type clientConfig struct {
	baseURL string
	token   string
}

type Hit struct {
	ID        int
	Name      string
	VideoName string
	Language  string
	Format    string
	Site      string
	Comment   string
}

type ClientConfig struct {
	BaseURL string
	Token   string
}

func NewClient(baseURL, token string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client := &Client{client: newAssrtHTTPClient(timeout)}
	client.Configure(baseURL, token)
	return client
}

func (c *Client) Configure(baseURL, token string) {
	c.mutex.Lock()
	c.config = clientConfig{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
	}
	c.lastCheck = integration.Health{}
	c.checkedAt = time.Time{}
	c.mutex.Unlock()
}

func (c *Client) Configured() bool {
	configuration := c.configuration()
	return configuration.baseURL != "" && configuration.token != ""
}

func (c *Client) configuration() clientConfig {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.config
}

func (c *Client) Check(ctx context.Context) integration.Health {
	health := integration.Health{ID: "assrt", Label: "Assrt"}
	if !c.Configured() {
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未配置中文字幕"
		return health
	}
	c.mutex.RLock()
	if !c.checkedAt.IsZero() && time.Since(c.checkedAt) < 2*time.Minute {
		cached := c.lastCheck
		c.mutex.RUnlock()
		return cached
	}
	c.mutex.RUnlock()
	if _, err := c.Search(ctx, "assrt", false); err != nil {
		if errors.Is(err, ErrUnauthorized) {
			health.Status = integration.StatusDegraded
			health.Detail = "服务可达，但 Token 无效"
		} else {
			health.Status = integration.StatusUnavailable
			health.Detail = "无法读取 Assrt 状态"
		}
	} else {
		health.Status = integration.StatusHealthy
		health.Detail = "中文字幕搜索可用"
	}
	c.mutex.Lock()
	c.lastCheck = health
	c.checkedAt = time.Now()
	c.mutex.Unlock()
	return health
}

func (c *Client) Search(ctx context.Context, query string, fileName bool) ([]Hit, error) {
	configuration := c.configuration()
	if configuration.baseURL == "" || configuration.token == "" {
		return nil, ErrNotConfigured
	}
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 3 {
		return nil, ErrUpstreamResponse
	}
	values := url.Values{"q": {query}, "cnt": {"15"}, "filelist": {"1"}}
	if fileName {
		values.Set("no_muxer", "1")
	}
	var payload searchResponse
	if err := c.getJSON(ctx, configuration, "/v1/sub/search", values, &payload); err != nil {
		return nil, err
	}
	hits := make([]Hit, 0, len(payload.Sub.Subs))
	for _, item := range payload.Sub.Subs {
		if item.ID < 1 {
			continue
		}
		hits = append(hits, publicHit(item))
		if len(hits) >= 20 {
			break
		}
	}
	return hits, nil
}

func (c *Client) DownloadFile(ctx context.Context, subtitleID int) (string, []byte, error) {
	configuration := c.configuration()
	if configuration.baseURL == "" || configuration.token == "" {
		return "", nil, ErrNotConfigured
	}
	if subtitleID < 1 {
		return "", nil, ErrUpstreamResponse
	}
	var payload detailResponse
	if err := c.getJSON(ctx, configuration, "/v1/sub/detail", url.Values{"id": {strconv.Itoa(subtitleID)}}, &payload); err != nil {
		return "", nil, err
	}
	if len(payload.Sub.Subs) == 0 {
		return "", nil, ErrUpstreamResponse
	}
	item := payload.Sub.Subs[0]
	fileURL, fileName := pickSubtitleFile(item)
	if fileURL == "" {
		return "", nil, ErrUnsupportedFile
	}
	body, err := c.getBytes(ctx, fileURL)
	if err != nil {
		return "", nil, err
	}
	name, data, err := extractSubtitle(fileName, body)
	if err != nil {
		return "", nil, err
	}
	return name, data, nil
}

type searchResponse struct {
	Status int `json:"status"`
	Sub    struct {
		Subs []subtitleInfo `json:"subs"`
	} `json:"sub"`
}

type detailResponse struct {
	Status int `json:"status"`
	Sub    struct {
		Subs []subtitleDetail `json:"subs"`
	} `json:"sub"`
}

type subtitleInfo struct {
	ID          int    `json:"id"`
	NativeName  string `json:"native_name"`
	VideoName   string `json:"videoname"`
	Subtype     string `json:"subtype"`
	ReleaseSite string `json:"release_site"`
	Lang        struct {
		Desc     string          `json:"desc"`
		LangList map[string]bool `json:"langlist"`
	} `json:"lang"`
}

type subtitleDetail struct {
	subtitleInfo
	URL      string `json:"url"`
	FileList []struct {
		URL  string `json:"url"`
		Name string `json:"f"`
	} `json:"filelist"`
}

func publicHit(item subtitleInfo) Hit {
	name := strings.TrimSpace(item.NativeName)
	if name == "" {
		name = strings.TrimSpace(item.VideoName)
	}
	return Hit{
		ID:        item.ID,
		Name:      bounded(name, 240),
		VideoName: bounded(item.VideoName, 240),
		Language:  languageCode(item.Lang.LangList, item.Lang.Desc),
		Format:    formatName(item.Subtype),
		Site:      bounded(item.ReleaseSite, 80),
		Comment:   bounded(item.Lang.Desc, 80),
	}
}

func languageCode(list map[string]bool, desc string) string {
	switch {
	case list["langchs"]:
		return "chi"
	case list["langcht"]:
		return "cht"
	case strings.Contains(desc, "简") || strings.Contains(desc, "繁") || strings.Contains(desc, "中"):
		return "chi"
	default:
		return "chi"
	}
}

func formatName(subtype string) string {
	lower := strings.ToLower(subtype)
	switch {
	case strings.Contains(lower, "ass"):
		return "ass"
	case strings.Contains(lower, "ssa"):
		return "ssa"
	case strings.Contains(lower, "srt") || strings.Contains(lower, "subrip"):
		return "srt"
	default:
		return bounded(strings.TrimSpace(subtype), 32)
	}
}

func pickSubtitleFile(item subtitleDetail) (string, string) {
	bestURL, bestName, bestScore := "", "", -1
	for _, file := range item.FileList {
		score := subtitleFileScore(file.Name)
		if score > bestScore {
			bestScore = score
			bestURL = strings.TrimSpace(file.URL)
			bestName = strings.TrimSpace(file.Name)
		}
	}
	if bestScore >= 50 && bestURL != "" {
		return bestURL, bestName
	}
	if url := strings.TrimSpace(item.URL); url != "" && subtitleFileScore(item.NativeName+item.VideoName) >= 0 {
		return url, fallbackName(item)
	}
	if bestURL != "" {
		return bestURL, bestName
	}
	return strings.TrimSpace(item.URL), fallbackName(item)
}

func fallbackName(item subtitleDetail) string {
	if name := strings.TrimSpace(item.VideoName); name != "" {
		return name
	}
	return strings.TrimSpace(item.NativeName)
}

func subtitleFileScore(name string) int {
	lower := strings.ToLower(name)
	ext := ""
	if index := strings.LastIndex(lower, "."); index >= 0 {
		ext = lower[index:]
	}
	score := 0
	switch ext {
	case ".ass", ".ssa":
		score = 80
	case ".srt":
		score = 70
	case ".zip":
		score = 20
	default:
		if ext == ".rar" || ext == ".7z" {
			return -1
		}
		return 0
	}
	if strings.Contains(lower, "chs") || strings.Contains(name, "简") {
		score += 10
	}
	if strings.Contains(lower, "cht") || strings.Contains(name, "繁") {
		score += 6
	}
	return score
}

func (c *Client) getJSON(ctx context.Context, configuration clientConfig, endpointPath string, query url.Values, target any) error {
	endpoint, err := url.Parse(configuration.baseURL + endpointPath)
	if err != nil {
		return ErrUpstreamResponse
	}
	values := url.Values{}
	for key, items := range query {
		values[key] = append([]string(nil), items...)
	}
	values.Set("token", configuration.token)
	endpoint.RawQuery = values.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("create Assrt request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+configuration.token)
	request.Header.Set("User-Agent", "Media-Hub/assrt")
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("request Assrt: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return fmt.Errorf("read Assrt response: %w", err)
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	var envelope struct {
		Status int `json:"status"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return ErrUpstreamResponse
		}
		return fmt.Errorf("decode Assrt response: %w", err)
	}
	if envelope.Status == 1 || envelope.Status == 20001 {
		return ErrUnauthorized
	}
	if envelope.Status != 0 || response.StatusCode < 200 || response.StatusCode >= 300 {
		return ErrUpstreamResponse
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode Assrt response: %w", err)
	}
	return nil
}

func (c *Client) getBytes(ctx context.Context, rawURL string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create Assrt download: %w", err)
	}
	request.Header.Set("User-Agent", "Media-Hub/assrt")
	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download Assrt file: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, ErrUpstreamResponse
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20+1))
	if err != nil {
		return nil, fmt.Errorf("read Assrt file: %w", err)
	}
	if len(body) > 8<<20 {
		return nil, ErrUpstreamResponse
	}
	return body, nil
}

func bounded(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}
