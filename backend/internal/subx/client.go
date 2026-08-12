package subx

import (
	"bytes"
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

	"media-hub/backend/internal/config"
	"media-hub/backend/internal/integration"
)

var (
	ErrNotConfigured     = errors.New("subx is not configured")
	ErrUnknownOperation  = errors.New("unknown subx operation")
	ErrInvalidInvocation = errors.New("invalid subx invocation")
	ErrUnauthorized      = errors.New("subx authentication failed")
	ErrUnavailable       = errors.New("subx is unavailable")
	ErrInvalidResponse   = errors.New("subx returned an invalid response")
)

const (
	maxRequestBytes  = 64 << 10
	maxResponseBytes = 4 << 20
)

type Invocation struct {
	PathParams map[string]string `json:"pathParams"`
	Query      map[string]string `json:"query"`
	Body       json.RawMessage   `json:"body"`
	Confirm    string            `json:"confirm,omitempty"`
}

type Client struct {
	baseURL  string
	username string
	password string
	static   string
	client   *http.Client
	mu       sync.Mutex
	token    string
}

func NewClient(configuration config.SubX, timeout time.Duration) *Client {
	return &Client{
		baseURL: configuration.BaseURL, username: configuration.Username,
		password: configuration.Password, static: configuration.Token,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *Client) Check(ctx context.Context) integration.Health {
	health := integration.Health{ID: "subx", Label: "SubX 兼容层"}
	if !c.Configured() {
		health.Status = integration.StatusUnconfigured
		health.Detail = "未配置"
		return health
	}
	if _, err := c.Read(ctx, "health", Invocation{}); err != nil {
		health.Status = integration.StatusUnavailable
		health.Detail = "连接失败"
		if errors.Is(err, ErrUnauthorized) {
			health.Status = integration.StatusDegraded
			health.Detail = "鉴权失败"
		}
		return health
	}
	health.Status = integration.StatusHealthy
	health.Detail = "兼容能力可用"
	return health
}

func (c *Client) Configured() bool {
	return c != nil && c.baseURL != "" && (c.static != "" || (c.username != "" && c.password != ""))
}

func (c *Client) Read(ctx context.Context, operationID string, invocation Invocation) (json.RawMessage, error) {
	operation, ok := LookupOperation(operationID)
	if !ok || operation.Command {
		return nil, ErrUnknownOperation
	}
	if err := validateInvocation(operation, invocation); err != nil {
		return nil, err
	}
	return c.execute(ctx, operation, invocation, true)
}

func (c *Client) ExecuteCommand(ctx context.Context, operationID string, invocation Invocation) (json.RawMessage, error) {
	operation, ok := LookupOperation(operationID)
	if !ok || !operation.Command {
		return nil, ErrUnknownOperation
	}
	if err := validateInvocation(operation, invocation); err != nil {
		return nil, err
	}
	return c.execute(ctx, operation, invocation, false)
}

func (c *Client) readInternal(ctx context.Context, operationID string, invocation Invocation) (json.RawMessage, error) {
	operation, ok := LookupOperation(operationID)
	if !ok || operation.Command {
		return nil, ErrUnknownOperation
	}
	if err := validateInvocation(operation, invocation); err != nil {
		return nil, err
	}
	return c.execute(ctx, operation, invocation, false)
}

func (c *Client) execute(ctx context.Context, operation Operation, invocation Invocation, sanitizeResponse bool) (json.RawMessage, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	endpoint, err := c.endpoint(operation, invocation)
	if err != nil {
		return nil, ErrInvalidInvocation
	}
	var body io.Reader
	if operation.Body {
		body = bytes.NewReader(invocation.Body)
	}
	request, err := http.NewRequestWithContext(ctx, operation.Method, endpoint, body)
	if err != nil {
		return nil, ErrInvalidInvocation
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("User-Agent", "Media-Hub/SubX-compat")
	if operation.Body {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(data) > maxResponseBytes {
		return nil, ErrInvalidResponse
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		c.clearCachedToken()
		return nil, ErrUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if response.StatusCode >= 500 || response.StatusCode == http.StatusTooManyRequests {
			return nil, ErrUnavailable
		}
		return nil, ErrInvalidInvocation
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return json.RawMessage(`{}`), nil
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, ErrInvalidResponse
	}
	if sanitizeResponse {
		value = sanitize(value, 0)
	}
	result, err := json.Marshal(value)
	if err != nil {
		return nil, ErrInvalidResponse
	}
	return result, nil
}

func (c *Client) endpoint(operation Operation, invocation Invocation) (string, error) {
	path := operation.Path
	for _, parameter := range operation.PathParams {
		value := invocation.PathParams[parameter.Name]
		path = strings.ReplaceAll(path, "{"+parameter.Name+"}", url.PathEscape(value))
	}
	parsed, err := url.Parse(c.baseURL + path)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	for _, parameter := range operation.Query {
		if value := invocation.Query[parameter.Name]; value != "" {
			query.Set(parameter.Name, value)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	if c.static != "" {
		return c.static, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" {
		return c.token, nil
	}
	body, _ := json.Marshal(map[string]string{"username": c.username, "password": c.password})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/auth/login", bytes.NewReader(body))
	if err != nil {
		return "", ErrUnauthorized
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Media-Hub/SubX-compat")
	response, err := c.client.Do(request)
	if err != nil {
		return "", ErrUnavailable
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return "", ErrInvalidResponse
	}
	if response.StatusCode != http.StatusOK {
		return "", ErrUnauthorized
	}
	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || strings.TrimSpace(payload.AccessToken) == "" {
		return "", ErrInvalidResponse
	}
	c.token = payload.AccessToken
	return c.token, nil
}

func (c *Client) clearCachedToken() {
	if c.static != "" {
		return
	}
	c.mu.Lock()
	c.token = ""
	c.mu.Unlock()
}

func validateInvocation(operation Operation, invocation Invocation) error {
	if operation.Destructive && invocation.Confirm != operation.ID {
		return ErrInvalidInvocation
	}
	if err := validateParameters(operation.PathParams, invocation.PathParams); err != nil {
		return err
	}
	if err := validateParameters(operation.Query, invocation.Query); err != nil {
		return err
	}
	body := bytes.TrimSpace(invocation.Body)
	if operation.Body {
		if len(body) == 0 || len(body) > maxRequestBytes || !json.Valid(body) {
			return ErrInvalidInvocation
		}
		var object map[string]any
		if err := json.Unmarshal(body, &object); err != nil || object == nil {
			return ErrInvalidInvocation
		}
	} else if len(body) > 0 && !bytes.Equal(body, []byte("null")) {
		return ErrInvalidInvocation
	}
	return nil
}

func validateParameters(definitions []Parameter, values map[string]string) error {
	if len(values) > len(definitions) {
		return ErrInvalidInvocation
	}
	allowed := make(map[string]Parameter, len(definitions))
	for _, definition := range definitions {
		allowed[definition.Name] = definition
		if definition.Required && strings.TrimSpace(values[definition.Name]) == "" {
			return ErrInvalidInvocation
		}
	}
	for name, raw := range values {
		definition, ok := allowed[name]
		value := strings.TrimSpace(raw)
		if !ok || len(value) > 1024 {
			return ErrInvalidInvocation
		}
		switch definition.Kind {
		case "integer":
			if _, err := strconv.ParseInt(value, 10, 64); err != nil {
				return ErrInvalidInvocation
			}
		case "boolean":
			if _, err := strconv.ParseBool(value); err != nil {
				return ErrInvalidInvocation
			}
		case "string":
		default:
			return ErrInvalidInvocation
		}
	}
	return nil
}

func sanitize(value any, depth int) any {
	if depth > 64 {
		return "[redacted]"
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			if sensitiveKey(key) {
				result[key] = "[redacted]"
				continue
			}
			result[key] = sanitize(child, depth+1)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = sanitize(child, depth+1)
		}
		return result
	default:
		return value
	}
}

func sensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	if normalized == "path" || normalized == "file_path" || normalized == "folder_path" || normalized == "share_link" {
		return true
	}
	for _, fragment := range []string{
		"password", "passwd", "secret", "token", "cookie", "authorization",
		"access_key", "api_key", "share_code", "receive_code", "pickcode", "direct_url",
	} {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

func (c *Client) String() string {
	return fmt.Sprintf("SubX compatibility client configured=%t", c.Configured())
}
