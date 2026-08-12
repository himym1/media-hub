package drive115

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

const userInfoEndpoint = "https://proapi.115.com/open/user/info"

var (
	ErrNotConfigured    = errors.New("115 access token is not configured")
	ErrUnauthorized     = errors.New("115 rejected authentication")
	ErrUpstreamResponse = errors.New("115 returned an invalid response")
)

type Client struct {
	mu          sync.RWMutex
	accessToken string
	endpoint    string
	client      *http.Client
}

type Status struct {
	Authorized  bool   `json:"authorized"`
	UsedBytes   int64  `json:"usedBytes,omitempty"`
	TotalBytes  int64  `json:"totalBytes,omitempty"`
	MemberLevel string `json:"memberLevel,omitempty"`
	ExpiresAt   int64  `json:"expiresAt,omitempty"`
}

type userInfoResponse struct {
	Code int `json:"code"`
	Data struct {
		Space struct {
			Used struct {
				Size int64 `json:"size"`
			} `json:"all_use"`
			Total struct {
				Size int64 `json:"size"`
			} `json:"all_total"`
		} `json:"rt_space_info"`
		VIP struct {
			LevelName string `json:"level_name"`
			Expire    int64  `json:"expire"`
		} `json:"vip_info"`
	} `json:"data"`
}

func NewClient(accessToken string, timeout time.Duration) *Client {
	return &Client{
		accessToken: accessToken,
		endpoint:    userInfoEndpoint,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *Client) SetAccessToken(value string) {
	c.mu.Lock()
	c.accessToken = value
	c.mu.Unlock()
}

func (c *Client) token() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.accessToken
}

func (c *Client) Check(ctx context.Context) integration.Health {
	health := integration.Health{ID: "115", Label: "115"}
	if c.token() == "" {
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未配置开放平台授权"
		return health
	}

	status, err := c.Status(ctx)
	if err == nil && status.Authorized {
		health.Status = integration.StatusHealthy
		if status.MemberLevel == "" {
			health.Detail = "开放平台授权正常"
		} else {
			health.Detail = status.MemberLevel + "授权正常"
		}
		return health
	}
	if errors.Is(err, ErrUnauthorized) {
		health.Status = integration.StatusDegraded
		health.Detail = "开放平台授权已失效"
		return health
	}
	health.Status = integration.StatusUnavailable
	health.Detail = "无法读取 115 授权状态"
	return health
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	accessToken := c.token()
	if accessToken == "" {
		return Status{}, ErrNotConfigured
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return Status{}, fmt.Errorf("create 115 request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("User-Agent", "Media-Hub/115-open")

	response, err := c.client.Do(request)
	if err != nil {
		return Status{}, fmt.Errorf("request 115: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return Status{}, ErrUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Status{}, ErrUpstreamResponse
	}

	var payload userInfoResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 2<<20))
	if err := decoder.Decode(&payload); err != nil {
		return Status{}, fmt.Errorf("decode 115 response: %w", err)
	}
	if payload.Code != 0 {
		return Status{}, ErrUnauthorized
	}
	return Status{
		Authorized:  true,
		UsedBytes:   payload.Data.Space.Used.Size,
		TotalBytes:  payload.Data.Space.Total.Size,
		MemberLevel: payload.Data.VIP.LevelName,
		ExpiresAt:   payload.Data.VIP.Expire,
	}, nil
}

type FileItem struct {
	ID        string `json:"id"`
	ParentID  string `json:"parentId"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Size      int64  `json:"size"`
	UpdatedAt int64  `json:"updatedAt"`
}

func (c *Client) ListFiles(ctx context.Context, parentID string, limit, offset int) ([]FileItem, int, error) {
	accessToken := c.token()
	if accessToken == "" {
		return nil, 0, ErrNotConfigured
	}
	if parentID == "" {
		parentID = "0"
	}
	if limit < 1 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	parsed, err := url.Parse("https://proapi.115.com/open/ufile/files")
	if err != nil {
		return nil, 0, err
	}
	query := parsed.Query()
	query.Set("aid", "1")
	query.Set("cid", parentID)
	query.Set("limit", strconv.Itoa(limit))
	query.Set("offset", strconv.Itoa(offset))
	query.Set("custom_order", "2")
	query.Set("o", "filename")
	query.Set("asc", "1")
	parsed.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("User-Agent", "Media-Hub/115-open")
	response, err := c.client.Do(request)
	if err != nil {
		return nil, 0, fmt.Errorf("list 115 files: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, 0, ErrUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, 0, ErrUpstreamResponse
	}
	var payload struct {
		State bool `json:"state"`
		Count int  `json:"count"`
		Data  []struct {
			ID       string      `json:"fid"`
			ParentID string      `json:"pid"`
			Name     string      `json:"fn"`
			Category string      `json:"fc"`
			Size     json.Number `json:"fs"`
			Updated  json.Number `json:"upt"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 4<<20))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, 0, ErrUpstreamResponse
	}
	items := make([]FileItem, 0, len(payload.Data))
	for _, value := range payload.Data {
		if value.ID == "" || value.Name == "" {
			continue
		}
		kind := "file"
		if value.Category == "0" {
			kind = "folder"
		}
		size, _ := value.Size.Int64()
		updated, _ := value.Updated.Int64()
		items = append(items, FileItem{ID: value.ID, ParentID: value.ParentID, Name: value.Name, Kind: kind, Size: size, UpdatedAt: updated})
	}
	return items, payload.Count, nil
}

type WriteError struct {
	Uncertain bool
	Code      string
	Err       error
}

func (e *WriteError) Error() string { return e.Err.Error() }

func (c *Client) ExecuteFileCommand(ctx context.Context, operation string, input map[string]any) error {
	values := url.Values{}
	endpoint := ""
	switch operation {
	case "create_folder":
		endpoint = "https://proapi.115.com/open/folder/add"
		values.Set("pid", stringValue(input["parentId"]))
		values.Set("file_name", stringValue(input["name"]))
	case "move":
		endpoint = "https://proapi.115.com/open/ufile/move"
		values.Set("file_ids", strings.Join(stringSlice(input["fileIds"]), ","))
		values.Set("to_cid", stringValue(input["targetParentId"]))
	case "rename":
		endpoint = "https://proapi.115.com/open/ufile/update"
		values.Set("file_id", stringValue(input["fileId"]))
		values.Set("file_name", stringValue(input["name"]))
	case "delete":
		endpoint = "https://proapi.115.com/open/ufile/delete"
		values.Set("file_ids", strings.Join(stringSlice(input["fileIds"]), ","))
	default:
		return &WriteError{Code: "unsupported_operation", Err: errors.New("unsupported 115 operation")}
	}
	return c.executeForm(ctx, endpoint, values)
}

func (c *Client) executeForm(ctx context.Context, endpoint string, values url.Values) error {
	accessToken := c.token()
	if accessToken == "" {
		return ErrNotConfigured
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return &WriteError{Code: "invalid_request", Err: err}
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Media-Hub/115-open")
	response, err := c.client.Do(request)
	if err != nil {
		return &WriteError{Uncertain: true, Code: "uncertain_result", Err: fmt.Errorf("submit 115 command: %w", err)}
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &WriteError{Code: "provider_rejected", Err: ErrUpstreamResponse}
	}
	var payload struct {
		State   bool   `json:"state"`
		Code    any    `json:"code"`
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return &WriteError{Uncertain: true, Code: "invalid_response", Err: err}
	}
	if !payload.State {
		return &WriteError{Code: "provider_rejected", Err: ErrUpstreamResponse}
	}
	return nil
}

func stringValue(value any) string {
	result, _ := value.(string)
	return result
}

func stringSlice(value any) []string {
	values, ok := value.([]string)
	if ok {
		return values
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if value, ok := item.(string); ok {
			result = append(result, value)
		}
	}
	return result
}
