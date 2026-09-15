package subtitlecat

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"media-hub/backend/internal/integration"
)

const (
	IDPrefix       = "subtitlecat:"
	defaultBaseURL = "https://www.subtitlecat.com"
	searchBudget   = 12 * time.Second
	maxBodyBytes   = 8 << 20
	minInterval    = 800 * time.Millisecond
)

var (
	ErrNotConfigured    = errors.New("Subtitlecat is not configured")
	ErrUpstreamResponse = errors.New("Subtitlecat returned an invalid response")
	ErrNotFound         = errors.New("Subtitlecat subtitle was not found")
)

type Hit struct {
	ID       string
	Name     string
	Language string
	Format   string
	PageURL  string
	FileURL  string
}

type Client struct {
	mutex     sync.Mutex
	baseURL   string
	client    *http.Client
	lastCall  time.Time
	lastCheck integration.Health
	checkedAt time.Time
}

func NewClient(timeout time.Duration, proxyURL *url.URL) *Client {
	if timeout <= 0 {
		timeout = searchBudget
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return &Client{
		baseURL: defaultBaseURL,
		client:  &http.Client{Timeout: timeout, Transport: transport},
	}
}

func (c *Client) Configured() bool {
	return c != nil && c.client != nil
}

func (c *Client) Check(ctx context.Context) integration.Health {
	health := integration.Health{ID: "subtitlecat", Label: "Subtitlecat"}
	if !c.Configured() {
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未启用 Subtitlecat"
		return health
	}
	c.mutex.Lock()
	if !c.checkedAt.IsZero() && time.Since(c.checkedAt) < 2*time.Minute {
		cached := c.lastCheck
		c.mutex.Unlock()
		return cached
	}
	c.mutex.Unlock()
	_, err := c.get(ctx, c.baseURL+"/")
	if err != nil {
		health.Status = integration.StatusUnavailable
		health.Detail = "无法读取 Subtitlecat"
	} else {
		health.Status = integration.StatusHealthy
		health.Detail = "本地字幕搜索可用"
	}
	c.mutex.Lock()
	c.lastCheck = health
	c.checkedAt = time.Now()
	c.mutex.Unlock()
	return health
}

func (c *Client) Search(ctx context.Context, query string) ([]Hit, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 {
		return nil, nil
	}
	searchCtx, cancel := context.WithTimeout(ctx, searchBudget)
	defer cancel()
	searchURL := c.baseURL + "/index.php?search=" + url.QueryEscape(query)
	body, err := c.get(searchCtx, searchURL)
	if err != nil {
		return nil, err
	}
	pages := parseSearchPages(c.baseURL, body)
	results := make([]Hit, 0, 12)
	seen := make(map[string]struct{})
	for _, page := range pages {
		detail, err := c.get(searchCtx, page)
		if err != nil {
			return nil, err
		}
		for _, hit := range parseDetailHits(c.baseURL, page, detail) {
			if _, exists := seen[hit.FileURL]; exists {
				continue
			}
			seen[hit.FileURL] = struct{}{}
			results = append(results, hit)
		}
		if len(results) >= 15 {
			break
		}
	}
	sortHits(results)
	if len(results) > 15 {
		results = results[:15]
	}
	return results, nil
}

func (c *Client) Download(ctx context.Context, id string) (string, []byte, error) {
	if !c.Configured() {
		return "", nil, ErrNotConfigured
	}
	fileURL, err := decodeID(c.baseURL, id)
	if err != nil {
		return "", nil, ErrNotFound
	}
	body, err := c.get(ctx, fileURL)
	if err != nil {
		return "", nil, err
	}
	if !looksLikeSubtitle(body) {
		return "", nil, ErrUpstreamResponse
	}
	name := fileURL
	if parsed, err := url.Parse(fileURL); err == nil {
		name = parsed.Path
	}
	return name, body, nil
}

func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	c.throttle()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Media-Hub/subtitlecat)")
	request.Header.Set("Accept", "text/html,application/octet-stream,*/*")
	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, ErrUpstreamResponse
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxBodyBytes {
		return nil, ErrUpstreamResponse
	}
	return body, nil
}

func (c *Client) throttle() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.lastCall.IsZero() {
		c.lastCall = time.Now()
		return
	}
	wait := minInterval - time.Since(c.lastCall)
	if wait > 0 {
		time.Sleep(wait)
	}
	c.lastCall = time.Now()
}

func encodeID(fileURL string) string {
	return IDPrefix + base64.RawURLEncoding.EncodeToString([]byte(fileURL))
}

func decodeID(baseURL, id string) (string, error) {
	raw := strings.TrimSpace(id)
	raw = strings.TrimPrefix(raw, IDPrefix)
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) == 0 {
		return "", fmt.Errorf("invalid subtitle id")
	}
	fileURL := string(decoded)
	root := strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/"
	if !strings.HasPrefix(fileURL, root) {
		return "", fmt.Errorf("invalid subtitle id")
	}
	return fileURL, nil
}

func looksLikeSubtitle(body []byte) bool {
	text := strings.ToLower(string(body[:min(len(body), 2048)]))
	if strings.Contains(text, "<html") || strings.Contains(text, "<!doctype") {
		return false
	}
	return strings.Contains(text, "-->") || strings.Contains(text, "[script info]") || strings.Contains(text, "dialogue:")
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
