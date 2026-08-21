package wecom

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
	"strings"
	"sync"
	"time"

	"media-hub/backend/internal/config"
	"media-hub/backend/internal/integration"
)

var (
	ErrNotConfigured     = errors.New("WeCom is not configured")
	ErrSubmissionUnknown = errors.New("WeCom message submission result is unknown")
	ErrRejected          = errors.New("WeCom rejected the message")
)

type SubmissionError struct {
	Unknown bool
	Code    int
	Err     error
}

func ErrorCode(err error) int {
	var failure SubmissionError
	if errors.As(err, &failure) {
		return failure.Code
	}
	return 0
}

func (failure SubmissionError) Error() string { return failure.Err.Error() }
func (failure SubmissionError) Unwrap() error { return failure.Err }

type clientConfig struct {
	baseURL  string
	corpID   string
	secret   string
	sendMode string
	agentID  uint
	toUser   string
	chatID   string
}

type Client struct {
	configMutex sync.RWMutex
	config      clientConfig
	client      *http.Client
	tokenMutex  sync.Mutex
	token       string
	expires     time.Time
	tokenConfig clientConfig
}

type apiResponse struct {
	ErrorCode   int    `json:"errcode"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

func NewClient(baseURL, corpID, secret, chatID string, timeout time.Duration) *Client {
	return NewConfiguredClient(config.WeCom{
		BaseURL: baseURL, CorpID: corpID, Secret: secret, SendMode: config.WeComSendModeAppChat, ChatID: chatID,
	}, timeout)
}

func NewConfiguredClient(configuration config.WeCom, timeout time.Duration) *Client {
	client := &Client{
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	client.ConfigureDelivery(configuration)
	return client
}

func (c *Client) Configure(baseURL, corpID, secret, chatID string) {
	c.ConfigureDelivery(config.WeCom{
		BaseURL: baseURL, CorpID: corpID, Secret: secret, SendMode: config.WeComSendModeAppChat, ChatID: chatID,
	})
}

func (c *Client) ConfigureDelivery(configuration config.WeCom) {
	c.configMutex.Lock()
	c.config = clientConfig{
		baseURL:  strings.TrimRight(strings.TrimSpace(configuration.BaseURL), "/"),
		corpID:   strings.TrimSpace(configuration.CorpID),
		secret:   strings.TrimSpace(configuration.Secret),
		sendMode: configuration.DeliveryMode(),
		agentID:  configuration.AgentID,
		toUser:   strings.TrimSpace(configuration.ToUser),
		chatID:   strings.TrimSpace(configuration.ChatID),
	}
	c.configMutex.Unlock()
	c.invalidateToken()
}

func (c *Client) configuration() clientConfig {
	c.configMutex.RLock()
	defer c.configMutex.RUnlock()
	return c.config
}

func (c *Client) Configured() bool {
	return configured(c.configuration())
}

func configured(configuration clientConfig) bool {
	if configuration.baseURL == "" || configuration.corpID == "" || configuration.secret == "" {
		return false
	}
	switch configuration.sendMode {
	case config.WeComSendModeApp:
		return configuration.agentID != 0 && configuration.toUser != "" && configuration.chatID == ""
	case config.WeComSendModeAppChat:
		return configuration.chatID != "" && configuration.agentID == 0 && configuration.toUser == ""
	default:
		return false
	}
}

func (c *Client) Check(ctx context.Context) integration.Health {
	health := integration.Health{ID: "wecom", Label: "企业微信"}
	configuration := c.configuration()
	if !configured(configuration) {
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未配置通知"
		return health
	}
	if _, err := c.accessToken(ctx, configuration); err != nil {
		health.Status = integration.StatusUnavailable
		health.Detail = "无法取得通知凭证"
		return health
	}
	health.Status = integration.StatusHealthy
	health.Detail = "通知凭证可用"
	return health
}

func (c *Client) Send(ctx context.Context, content string) (bool, error) {
	configuration := c.configuration()
	if !configured(configuration) {
		return false, SubmissionError{Err: ErrNotConfigured}
	}
	if len([]byte(content)) > 2048 {
		return false, SubmissionError{Err: ErrRejected}
	}
	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.accessToken(ctx, configuration)
		if err != nil {
			var failure SubmissionError
			if errors.As(err, &failure) {
				return failure.Unknown, failure
			}
			return false, SubmissionError{Err: err}
		}
		errorCode, err := c.send(ctx, configuration, token, content)
		if err != nil {
			var failure SubmissionError
			if errors.As(err, &failure) {
				return failure.Unknown, err
			}
			return false, err
		}
		if errorCode == 0 {
			return false, nil
		}
		if (errorCode == 40014 || errorCode == 42001) && attempt == 0 {
			c.invalidateTokenFor(configuration)
			continue
		}
		return false, SubmissionError{Code: errorCode, Err: ErrRejected}
	}
	return false, SubmissionError{Err: ErrRejected}
}

func (c *Client) accessToken(ctx context.Context, configuration clientConfig) (string, error) {
	c.tokenMutex.Lock()
	defer c.tokenMutex.Unlock()
	if c.token != "" && c.tokenConfig == configuration && time.Now().UTC().Add(time.Minute).Before(c.expires) {
		return c.token, nil
	}
	endpoint, err := endpointURL(configuration, "gettoken", url.Values{"corpid": {configuration.corpID}, "corpsecret": {configuration.secret}})
	if err != nil {
		return "", ErrNotConfigured
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create WeCom token request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Media-Hub/wecom")
	var response apiResponse
	if err := c.do(request, &response, false); err != nil {
		return "", err
	}
	if response.ErrorCode != 0 || response.AccessToken == "" || response.ExpiresIn < 60 {
		return "", SubmissionError{Code: response.ErrorCode, Err: ErrRejected}
	}
	expires := time.Now().UTC().Add(time.Duration(response.ExpiresIn) * time.Second)
	if c.configuration() == configuration {
		c.token = response.AccessToken
		c.expires = expires
		c.tokenConfig = configuration
	}
	return response.AccessToken, nil
}

func (c *Client) send(ctx context.Context, configuration clientConfig, token, content string) (int, error) {
	endpointPath := "appchat/send"
	var body []byte
	var err error
	if configuration.sendMode == config.WeComSendModeApp {
		endpointPath = "message/send"
		body, err = json.Marshal(struct {
			ToUser  string `json:"touser"`
			MsgType string `json:"msgtype"`
			AgentID uint   `json:"agentid"`
			Text    struct {
				Content string `json:"content"`
			} `json:"text"`
		}{ToUser: configuration.toUser, MsgType: "text", AgentID: configuration.agentID, Text: struct {
			Content string `json:"content"`
		}{Content: content}})
	} else {
		body, err = json.Marshal(struct {
			ChatID  string `json:"chatid"`
			MsgType string `json:"msgtype"`
			Text    struct {
				Content string `json:"content"`
			} `json:"text"`
		}{ChatID: configuration.chatID, MsgType: "text", Text: struct {
			Content string `json:"content"`
		}{Content: content}})
	}
	if err != nil {
		return 0, SubmissionError{Err: ErrRejected}
	}
	endpoint, err := endpointURL(configuration, endpointPath, url.Values{"access_token": {token}})
	if err != nil {
		return 0, SubmissionError{Err: ErrNotConfigured}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, SubmissionError{Err: ErrRejected}
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Media-Hub/wecom")
	var response apiResponse
	if err := c.do(request, &response, true); err != nil {
		return 0, err
	}
	return response.ErrorCode, nil
}

func (c *Client) do(request *http.Request, target any, submission bool) error {
	response, err := c.client.Do(request)
	if err != nil {
		if submission {
			return SubmissionError{Unknown: true, Err: ErrSubmissionUnknown}
		}
		return fmt.Errorf("request WeCom: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if submission {
			return SubmissionError{Unknown: true, Err: ErrSubmissionUnknown}
		}
		return ErrRejected
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 || json.Unmarshal(data, target) != nil {
		if submission {
			return SubmissionError{Unknown: true, Err: ErrSubmissionUnknown}
		}
		return ErrRejected
	}
	return nil
}

func (c *Client) invalidateToken() {
	c.tokenMutex.Lock()
	c.token = ""
	c.expires = time.Time{}
	c.tokenConfig = clientConfig{}
	c.tokenMutex.Unlock()
}

func (c *Client) invalidateTokenFor(configuration clientConfig) {
	c.tokenMutex.Lock()
	if c.tokenConfig == configuration {
		c.token = ""
		c.expires = time.Time{}
		c.tokenConfig = clientConfig{}
	}
	c.tokenMutex.Unlock()
}

func endpointURL(configuration clientConfig, endpointPath string, query url.Values) (string, error) {
	parsed, err := url.Parse(configuration.baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", ErrNotConfigured
	}
	parsed.Path = path.Join("/", parsed.Path, "cgi-bin", endpointPath)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
