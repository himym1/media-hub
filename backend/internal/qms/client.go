package qms

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
	"sync"
	"time"

	"media-hub/backend/internal/integration"
)

var (
	ErrNotConfigured     = errors.New("QMediaSync is not configured")
	ErrMissingAPIKey     = errors.New("QMediaSync API key is missing")
	ErrUnauthorized      = errors.New("QMediaSync rejected authentication")
	ErrUpstreamResponse  = errors.New("QMediaSync returned an invalid response")
	ErrRejected          = errors.New("QMediaSync rejected the sync request")
	ErrSubmissionUnknown = errors.New("QMediaSync submission result is unknown")
)

type clientConfig struct {
	baseURL string
	apiKey  string
}

type Client struct {
	mutex  sync.RWMutex
	config clientConfig
	client *http.Client
}

type Status struct {
	Version     string       `json:"version"`
	ReleaseDate string       `json:"releaseDate,omitempty"`
	RecentSyncs []SyncRecord `json:"recentSyncs"`
	TotalSyncs  int          `json:"totalSyncs"`
}

type SyncRecord struct {
	ID         string     `json:"id"`
	State      string     `json:"state"`
	TotalFiles int        `json:"totalFiles"`
	NewSTRM    int        `json:"newStrm"`
	NewMeta    int        `json:"newMetadata"`
	NewUploads int        `json:"newUploads"`
	CreatedAt  *time.Time `json:"createdAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	BaseCID    string     `json:"-"`
}

type versionResponse struct {
	Version string `json:"version"`
	Date    string `json:"date"`
}

type syncEnvelope struct {
	Code int `json:"code"`
	Data struct {
		Records []syncRecord `json:"records"`
		Total   int          `json:"total"`
	} `json:"data"`
}

type syncRecord struct {
	ID        uint   `json:"id"`
	Status    int    `json:"status"`
	Total     int    `json:"total"`
	NewSTRM   int    `json:"new_strm"`
	NewMeta   int    `json:"new_meta"`
	NewUpload int    `json:"new_upload"`
	BaseCID   string `json:"base_cid"`
	CreatedAt int64  `json:"created_at"`
	Finished  int64  `json:"finish_at"`
}

type ManualSyncRequest struct {
	PathID     string `json:"path_id"`
	Path       string `json:"path,omitempty"`
	TargetPath string `json:"target_path"`
	IsFile     bool   `json:"is_file"`
	AccountID  uint   `json:"account_id"`
}

type actionEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		config: clientConfig{baseURL: baseURL, apiKey: apiKey},
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *Client) Configure(baseURL, apiKey string) {
	c.mutex.Lock()
	c.config = clientConfig{baseURL: baseURL, apiKey: apiKey}
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
	health := integration.Health{ID: "qmediasync", Label: "QMediaSync"}
	configuration := c.configuration()
	if configuration.baseURL == "" {
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未配置服务地址"
		return health
	}
	if configuration.apiKey == "" {
		health.Status = integration.StatusDegraded
		health.Detail = "服务地址已配置，缺少 API Key"
		return health
	}

	version, err := c.version(ctx, configuration)
	if err == nil {
		health.Status = integration.StatusHealthy
		health.Detail = "QMediaSync " + version.Version + " 在线"
		return health
	}
	if errors.Is(err, ErrUnauthorized) {
		health.Status = integration.StatusDegraded
		health.Detail = "服务可达，但鉴权失败"
		return health
	}
	health.Status = integration.StatusUnavailable
	health.Detail = "无法读取 QMediaSync 状态"
	return health
}

func (c *Client) ReadStatus(ctx context.Context) (Status, error) {
	configuration := c.configuration()
	version, err := c.version(ctx, configuration)
	if err != nil {
		return Status{}, err
	}
	records, total, err := c.recentSyncs(ctx, configuration, 10)
	if err != nil {
		return Status{}, err
	}
	return Status{
		Version: version.Version, ReleaseDate: version.Date,
		RecentSyncs: records, TotalSyncs: total,
	}, nil
}

func (c *Client) Version(ctx context.Context) (versionResponse, error) {
	return c.version(ctx, c.configuration())
}

func (c *Client) version(ctx context.Context, configuration clientConfig) (versionResponse, error) {
	if err := validateClientConfiguration(configuration); err != nil {
		return versionResponse{}, err
	}
	var response versionResponse
	if err := c.getJSON(ctx, configuration, "api/version", nil, &response); err != nil {
		return versionResponse{}, err
	}
	if response.Version == "" {
		return versionResponse{}, ErrUpstreamResponse
	}
	return response, nil
}

func (c *Client) RecentSyncs(ctx context.Context, limit int) ([]SyncRecord, int, error) {
	return c.recentSyncs(ctx, c.configuration(), limit)
}

func (c *Client) recentSyncs(ctx context.Context, configuration clientConfig, limit int) ([]SyncRecord, int, error) {
	if err := validateClientConfiguration(configuration); err != nil {
		return nil, 0, err
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	query := url.Values{"page": {"1"}, "page_size": {strconv.Itoa(limit)}}
	var response syncEnvelope
	if err := c.getJSON(ctx, configuration, "api/sync/records", query, &response); err != nil {
		return nil, 0, err
	}
	if response.Code != http.StatusOK {
		return nil, 0, ErrUpstreamResponse
	}

	records := make([]SyncRecord, 0, len(response.Data.Records))
	for _, record := range response.Data.Records {
		records = append(records, SyncRecord{
			ID:         strconv.FormatUint(uint64(record.ID), 10),
			State:      syncState(record.Status),
			TotalFiles: record.Total,
			NewSTRM:    record.NewSTRM,
			NewMeta:    record.NewMeta,
			NewUploads: record.NewUpload,
			CreatedAt:  unixTime(record.CreatedAt),
			FinishedAt: unixTime(record.Finished),
			BaseCID:    record.BaseCID,
		})
	}
	return records, response.Data.Total, nil
}

func (c *Client) SubmitManualSync(ctx context.Context, input ManualSyncRequest) error {
	configuration := c.configuration()
	if err := validateClientConfiguration(configuration); err != nil {
		return err
	}
	body, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("encode QMediaSync request: %w", err)
	}
	endpoint, err := endpointURL(configuration, "api/sync/manual", nil)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create QMediaSync request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Media-Hub/qmediasync")
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("%w: transport failure", ErrSubmissionUnknown)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ErrSubmissionUnknown
	}
	var envelope actionEnvelope
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&envelope); err != nil {
		return ErrSubmissionUnknown
	}
	if envelope.Code != http.StatusOK {
		return ErrRejected
	}
	return nil
}

func (c *Client) FindSyncByBaseCID(ctx context.Context, baseCID string, createdAfter time.Time) (SyncRecord, bool, error) {
	configuration := c.configuration()
	records, _, err := c.recentSyncs(ctx, configuration, 50)
	if err != nil {
		return SyncRecord{}, false, err
	}
	for _, record := range records {
		if record.BaseCID != baseCID || record.CreatedAt == nil {
			continue
		}
		if record.CreatedAt.Before(createdAfter.Add(-5 * time.Second)) {
			continue
		}
		return record, true, nil
	}
	return SyncRecord{}, false, nil
}

func (c *Client) validateConfiguration() error {
	return validateClientConfiguration(c.configuration())
}

func validateClientConfiguration(configuration clientConfig) error {
	if configuration.baseURL == "" {
		return ErrNotConfigured
	}
	if configuration.apiKey == "" {
		return ErrMissingAPIKey
	}
	return nil
}

func (c *Client) getJSON(ctx context.Context, configuration clientConfig, endpointPath string, query url.Values, target any) error {
	endpoint, err := endpointURL(configuration, endpointPath, query)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create QMediaSync request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Media-Hub/qmediasync")

	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("request QMediaSync: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ErrUpstreamResponse
	}

	decoder := json.NewDecoder(io.LimitReader(response.Body, 2<<20))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode QMediaSync response: %w", err)
	}
	return nil
}

func endpointURL(configuration clientConfig, endpointPath string, query url.Values) (string, error) {
	if err := validateClientConfiguration(configuration); err != nil {
		return "", err
	}
	parsed, err := url.Parse(configuration.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse QMediaSync URL: %w", err)
	}
	parsed.Path = path.Join("/", parsed.Path, endpointPath)
	values := parsed.Query()
	for key, items := range query {
		for _, item := range items {
			values.Add(key, item)
		}
	}
	values.Set("api_key", configuration.apiKey)
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func syncState(status int) string {
	switch status {
	case 0:
		return "queued"
	case 1:
		return "running"
	case 2:
		return "completed"
	case 3:
		return "failed"
	default:
		return "unknown"
	}
}

func unixTime(value int64) *time.Time {
	if value <= 0 {
		return nil
	}
	parsed := time.Unix(value, 0).UTC()
	return &parsed
}
