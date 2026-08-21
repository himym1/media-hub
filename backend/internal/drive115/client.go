package drive115

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"media-hub/backend/internal/integration"
)

const (
	userAgent            = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36 115Browser/27.0.3.7"
	userInfoEndpoint     = "https://webapi.115.com/files/index_info"
	userProfileEndpoint  = "https://my.115.com/?ct=ajax&ac=nav"
	filesEndpoint        = "https://webapi.115.com/files"
	fileInfoEndpoint     = "https://webapi.115.com/files/get_info"
	downloadEndpoint     = "https://proapi.115.com/app/chrome/downurl"
	folderAddEndpoint    = "https://webapi.115.com/files/add"
	fileMoveEndpoint     = "https://webapi.115.com/files/move"
	fileRenameEndpoint   = "https://webapi.115.com/files/batch_rename"
	fileDeleteEndpoint   = "https://webapi.115.com/rb/delete"
	offlineInfoEndpoint  = "https://115.com/?ct=offline&ac=space"
	offlineAddEndpoint   = "https://115.com/web/lixian/?ct=lixian&ac=add_task_urls"
	shareSnapEndpoint    = "https://webapi.115.com/share/snap"
	shareReceiveEndpoint = "https://webapi.115.com/share/receive"
)

var (
	ErrNotConfigured    = errors.New("115 session is not configured")
	ErrUnauthorized     = errors.New("115 rejected authentication")
	ErrUpstreamResponse = errors.New("115 returned an invalid response")
)

type Client struct {
	mu              sync.RWMutex
	cookie          string
	userInfoURL     string
	userProfileURL  string
	filesURL        string
	fileInfoURL     string
	downloadURL     string
	folderAddURL    string
	fileMoveURL     string
	fileRenameURL   string
	fileDeleteURL   string
	offlineInfoURL  string
	offlineAddURL   string
	shareSnapURL    string
	shareReceiveURL string
	client          *http.Client
}

type Status struct {
	Authorized  bool   `json:"authorized"`
	UsedBytes   int64  `json:"usedBytes,omitempty"`
	TotalBytes  int64  `json:"totalBytes,omitempty"`
	MemberLevel string `json:"memberLevel,omitempty"`
	ExpiresAt   int64  `json:"expiresAt,omitempty"`
}

func NewClient(cookie string, timeout time.Duration) *Client {
	return &Client{
		cookie:          strings.TrimSpace(cookie),
		userInfoURL:     userInfoEndpoint,
		userProfileURL:  userProfileEndpoint,
		filesURL:        filesEndpoint,
		fileInfoURL:     fileInfoEndpoint,
		downloadURL:     downloadEndpoint,
		folderAddURL:    folderAddEndpoint,
		fileMoveURL:     fileMoveEndpoint,
		fileRenameURL:   fileRenameEndpoint,
		fileDeleteURL:   fileDeleteEndpoint,
		offlineInfoURL:  offlineInfoEndpoint,
		offlineAddURL:   offlineAddEndpoint,
		shareSnapURL:    shareSnapEndpoint,
		shareReceiveURL: shareReceiveEndpoint,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *Client) SetSession(value string) {
	c.mu.Lock()
	c.cookie = strings.TrimSpace(value)
	c.mu.Unlock()
}

func (c *Client) session() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cookie
}

func (c *Client) Check(ctx context.Context) integration.Health {
	health := integration.Health{ID: "115", Label: "115"}
	if c.session() == "" {
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未扫码授权"
		return health
	}
	status, err := c.Status(ctx)
	if err == nil && status.Authorized {
		health.Status = integration.StatusHealthy
		if status.MemberLevel == "" {
			health.Detail = "扫码授权正常"
		} else {
			health.Detail = status.MemberLevel + "授权正常"
		}
		return health
	}
	if errors.Is(err, ErrUnauthorized) {
		health.Status = integration.StatusDegraded
		health.Detail = "扫码授权已失效"
		return health
	}
	health.Status = integration.StatusUnavailable
	health.Detail = "无法读取 115 授权状态"
	return health
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	cookie := c.session()
	if cookie == "" {
		return Status{}, ErrNotConfigured
	}
	var payload struct {
		State bool   `json:"state"`
		Error string `json:"error"`
		Data  struct {
			Space struct {
				Used  byteSize `json:"all_use"`
				Total byteSize `json:"all_total"`
			} `json:"space_info"`
		} `json:"data"`
	}
	if err := c.getJSONWithSession(ctx, c.userInfoURL, nil, cookie, &payload); err != nil {
		return Status{}, err
	}
	if !payload.State {
		return Status{}, ErrUnauthorized
	}
	return Status{
		Authorized: true,
		UsedBytes:  payload.Data.Space.Used.Int64(),
		TotalBytes: payload.Data.Space.Total.Int64(),
	}, nil
}

type byteSize int64

func (value *byteSize) UnmarshalJSON(data []byte) error {
	var raw struct {
		Size json.RawMessage `json:"size"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw.Size) == 0 || string(raw.Size) == "null" {
		return nil
	}
	text := strings.TrimSpace(string(raw.Size))
	if len(text) > 0 && text[0] == '"' {
		if err := json.Unmarshal(raw.Size, &text); err != nil {
			return err
		}
	}
	number, _, err := big.ParseFloat(strings.TrimSpace(text), 10, 256, big.ToZero)
	if err != nil {
		return err
	}
	integer, _ := number.Int(nil)
	if integer == nil || !integer.IsInt64() || integer.Sign() < 0 {
		return ErrUpstreamResponse
	}
	*value = byteSize(integer.Int64())
	return nil
}

func (value byteSize) Int64() int64 { return int64(value) }

type FileItem struct {
	ID        string `json:"id"`
	ParentID  string `json:"parentId"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Size      int64  `json:"size"`
	UpdatedAt int64  `json:"updatedAt"`
}

func (c *Client) ListFiles(ctx context.Context, parentID string, limit, offset int) ([]FileItem, int, error) {
	cookie := c.session()
	if cookie == "" {
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
	query := url.Values{}
	query.Set("aid", "1")
	query.Set("cid", parentID)
	query.Set("o", "user_ptime")
	query.Set("asc", "0")
	query.Set("offset", strconv.Itoa(offset))
	query.Set("show_dir", "1")
	query.Set("limit", strconv.Itoa(limit))
	query.Set("format", "json")
	var payload struct {
		State bool `json:"state"`
		Count int  `json:"count"`
		Data  []struct {
			FileID   string          `json:"fid"`
			Category string          `json:"cid"`
			ParentID string          `json:"pid"`
			Name     string          `json:"n"`
			AltName  string          `json:"fn"`
			Size     json.Number     `json:"s"`
			AltSize  json.Number     `json:"fs"`
			Updated  json.RawMessage `json:"t"`
		} `json:"data"`
	}
	if err := c.getJSONWithSession(ctx, c.filesURL, query, cookie, &payload); err != nil {
		return nil, 0, err
	}
	if !payload.State {
		return nil, 0, ErrUnauthorized
	}
	items := make([]FileItem, 0, len(payload.Data))
	for _, value := range payload.Data {
		name := value.Name
		if name == "" {
			name = value.AltName
		}
		id := value.FileID
		kind := "file"
		parent := value.ParentID
		if value.FileID == "" || value.FileID == "0" {
			id = value.Category
			kind = "folder"
		} else if parent == "" {
			parent = value.Category
		}
		if id == "" || name == "" {
			continue
		}
		size, _ := value.Size.Int64()
		if size == 0 {
			size, _ = value.AltSize.Int64()
		}
		items = append(items, FileItem{ID: id, ParentID: parent, Name: name, Kind: kind, Size: size, UpdatedAt: parse115Time(value.Updated)})
	}
	return items, payload.Count, nil
}

func (c *Client) FolderPath(ctx context.Context, folderID string) (string, error) {
	folderID = strings.TrimSpace(folderID)
	if !numericIDPattern.MatchString(folderID) || folderID == "0" {
		return "", ErrUpstreamResponse
	}
	cookie := c.session()
	if cookie == "" {
		return "", ErrNotConfigured
	}
	query := url.Values{
		"aid": {"1"}, "cid": {folderID}, "offset": {"0"}, "limit": {"1"}, "show_dir": {"1"}, "format": {"json"},
	}
	var payload struct {
		State bool `json:"state"`
		Path  []struct {
			Name string          `json:"name"`
			CID  json.RawMessage `json:"cid"`
		} `json:"path"`
	}
	if err := c.getJSONWithSession(ctx, c.filesURL, query, cookie, &payload); err != nil {
		return "", err
	}
	if !payload.State {
		return "", ErrUnauthorized
	}
	parts := make([]string, 0, len(payload.Path))
	for _, item := range payload.Path {
		id := strings.Trim(strings.TrimSpace(string(item.CID)), `"`)
		if !numericIDPattern.MatchString(id) {
			return "", ErrUpstreamResponse
		}
		if id == "0" {
			continue
		}
		if item.Name == "" || strings.ContainsAny(item.Name, "/\x00") {
			return "", ErrUpstreamResponse
		}
		parts = append(parts, item.Name)
	}
	if len(parts) == 0 {
		return "", ErrUpstreamResponse
	}
	return strings.Join(parts, "/"), nil
}

func (c *Client) EnsureFolder(ctx context.Context, parentID, name string) (string, error) {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		parentID = "0"
	}
	name = sanitizeFolderName(name)
	if !numericIDPattern.MatchString(parentID) || name == "" {
		return "", &WriteError{Code: "invalid_request", Err: ErrUpstreamResponse}
	}
	for offset := 0; offset < 1000; {
		items, total, err := c.ListFiles(ctx, parentID, 100, offset)
		if err != nil {
			return "", err
		}
		for _, item := range items {
			if item.Kind == "folder" && item.Name == name {
				return item.ID, nil
			}
		}
		offset += len(items)
		if len(items) == 0 || offset >= total {
			break
		}
	}
	return c.CreateFolder(ctx, parentID, name)
}

func (c *Client) CreateFolder(ctx context.Context, parentID, name string) (string, error) {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		parentID = "0"
	}
	name = sanitizeFolderName(name)
	if !numericIDPattern.MatchString(parentID) || name == "" {
		return "", &WriteError{Code: "invalid_request", Err: ErrUpstreamResponse}
	}
	cookie := c.session()
	if cookie == "" {
		return "", ErrNotConfigured
	}
	values := url.Values{"pid": {parentID}, "cname": {name}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.folderAddURL, strings.NewReader(values.Encode()))
	if err != nil {
		return "", &WriteError{Code: "invalid_request", Err: err}
	}
	c.applyAuth(request, cookie)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.client.Do(request)
	if err != nil {
		return "", &WriteError{Uncertain: true, Code: "uncertain_result", Err: fmt.Errorf("create 115 folder: %w", err)}
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return "", ErrUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", &WriteError{Code: "provider_rejected", Err: ErrUpstreamResponse}
	}
	var payload struct {
		State bool            `json:"state"`
		CID   json.RawMessage `json:"cid"`
		Error string          `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return "", &WriteError{Uncertain: true, Code: "invalid_response", Err: err}
	}
	if !payload.State {
		return "", &WriteError{Code: "provider_rejected", Err: ErrUpstreamResponse}
	}
	folderID := strings.Trim(strings.TrimSpace(string(payload.CID)), `"`)
	if !numericIDPattern.MatchString(folderID) || folderID == "0" {
		return "", &WriteError{Uncertain: true, Code: "invalid_response", Err: ErrUpstreamResponse}
	}
	return folderID, nil
}

func sanitizeFolderName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	cleaned := strings.Map(func(character rune) rune {
		switch character {
		case '/', '\\', 0:
			return -1
		default:
			return character
		}
	}, name)
	cleaned = strings.TrimSpace(cleaned)
	runes := []rune(cleaned)
	if len(runes) > 200 {
		cleaned = string(runes[:200])
	}
	return strings.TrimSpace(cleaned)
}

type WriteError struct {
	Uncertain bool
	Code      string
	Err       error
}

func (e *WriteError) Error() string { return e.Err.Error() }

func (e *WriteError) SubmissionUncertain() bool {
	return e != nil && e.Uncertain
}

func (e *WriteError) AutomaticRetryAllowed() bool {
	return e != nil && e.Code != "provider_rejected" && e.Code != "invalid_request"
}

func (c *Client) ExecuteFileCommand(ctx context.Context, operation string, input map[string]any) error {
	values := url.Values{}
	endpoint := ""
	switch operation {
	case "create_folder":
		endpoint = c.folderAddURL
		values.Set("pid", stringValue(input["parentId"]))
		values.Set("cname", stringValue(input["name"]))
	case "move":
		endpoint = c.fileMoveURL
		for index, id := range stringSlice(input["fileIds"]) {
			values.Set("fid["+strconv.Itoa(index)+"]", id)
		}
		values.Set("pid", stringValue(input["targetParentId"]))
	case "rename":
		endpoint = c.fileRenameURL
		fileID := stringValue(input["fileId"])
		name := stringValue(input["name"])
		values.Set("fid", fileID)
		values.Set("file_name", name)
		values.Set("files_new_name["+fileID+"]", name)
	case "delete":
		endpoint = c.fileDeleteURL
		for index, id := range stringSlice(input["fileIds"]) {
			values.Set("fid["+strconv.Itoa(index)+"]", id)
		}
	default:
		return &WriteError{Code: "unsupported_operation", Err: errors.New("unsupported 115 operation")}
	}
	return c.executeForm(ctx, endpoint, values)
}

func (c *Client) executeForm(ctx context.Context, endpoint string, values url.Values) error {
	cookie := c.session()
	if cookie == "" {
		return ErrNotConfigured
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return &WriteError{Code: "invalid_request", Err: err}
	}
	c.applyAuth(request, cookie)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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

func (c *Client) getJSONWithSession(ctx context.Context, endpoint string, query url.Values, cookie string, target any) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	if query != nil {
		existing := parsed.Query()
		for key, values := range query {
			for _, value := range values {
				existing.Set(key, value)
			}
		}
		parsed.RawQuery = existing.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return err
	}
	c.applyAuth(request, cookie)
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("request 115: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ErrUpstreamResponse
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(target); err != nil {
		return ErrUpstreamResponse
	}
	return nil
}

func (c *Client) applyAuth(request *http.Request, cookie string) {
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", userAgent)
	if cookie != "" {
		request.Header.Set("Cookie", cookie)
	}
}

func parse115Time(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		value, _ := number.Int64()
		return value
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", text, time.Local)
	if err != nil {
		return 0
	}
	return parsed.Unix()
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
