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

	"media-hub/backend/internal/integration"
)

var (
	ErrNotConfigured     = errors.New("WeCom is not configured")
	ErrSubmissionUnknown = errors.New("WeCom message submission result is unknown")
	ErrRejected          = errors.New("WeCom rejected the message")
)

type SubmissionError struct {
	Unknown bool
	Err     error
}

func (failure SubmissionError) Error() string { return failure.Err.Error() }
func (failure SubmissionError) Unwrap() error { return failure.Err }

type Client struct {
	baseURL string
	corpID  string
	secret  string
	chatID  string
	client  *http.Client
	mutex   sync.Mutex
	token   string
	expires time.Time
}

type apiResponse struct {
	ErrorCode   int    `json:"errcode"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

func NewClient(baseURL, corpID, secret, chatID string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"), corpID: corpID, secret: secret, chatID: chatID,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *Client) Configured() bool {
	return c.baseURL != "" && c.corpID != "" && c.secret != "" && c.chatID != ""
}

func (c *Client) Check(ctx context.Context) integration.Health {
	health := integration.Health{ID: "wecom", Label: "企业微信"}
	if !c.Configured() {
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未配置通知"
		return health
	}
	if _, err := c.accessToken(ctx); err != nil {
		health.Status = integration.StatusUnavailable
		health.Detail = "无法取得通知凭证"
		return health
	}
	health.Status = integration.StatusHealthy
	health.Detail = "通知凭证可用"
	return health
}

func (c *Client) Send(ctx context.Context, content string) (bool, error) {
	if !c.Configured() {
		return false, SubmissionError{Err: ErrNotConfigured}
	}
	if len([]byte(content)) > 2048 {
		return false, SubmissionError{Err: ErrRejected}
	}
	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.accessToken(ctx)
		if err != nil {
			return false, SubmissionError{Err: err}
		}
		errorCode, err := c.send(ctx, token, content)
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
			c.invalidateToken()
			continue
		}
		return false, SubmissionError{Err: ErrRejected}
	}
	return false, SubmissionError{Err: ErrRejected}
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.token != "" && time.Now().UTC().Add(time.Minute).Before(c.expires) {
		return c.token, nil
	}
	endpoint, err := c.endpoint("gettoken", url.Values{"corpid": {c.corpID}, "corpsecret": {c.secret}})
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
		return "", ErrRejected
	}
	c.token = response.AccessToken
	c.expires = time.Now().UTC().Add(time.Duration(response.ExpiresIn) * time.Second)
	return c.token, nil
}

func (c *Client) send(ctx context.Context, token, content string) (int, error) {
	endpoint, err := c.endpoint("appchat/send", url.Values{"access_token": {token}})
	if err != nil {
		return 0, SubmissionError{Err: ErrNotConfigured}
	}
	body, err := json.Marshal(struct {
		ChatID  string `json:"chatid"`
		MsgType string `json:"msgtype"`
		Text    struct {
			Content string `json:"content"`
		} `json:"text"`
	}{ChatID: c.chatID, MsgType: "text", Text: struct {
		Content string `json:"content"`
	}{Content: content}})
	if err != nil {
		return 0, SubmissionError{Err: ErrRejected}
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
	c.mutex.Lock()
	c.token = ""
	c.expires = time.Time{}
	c.mutex.Unlock()
}

func (c *Client) endpoint(endpointPath string, query url.Values) (string, error) {
	parsed, err := url.Parse(c.baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", ErrNotConfigured
	}
	parsed.Path = path.Join("/", parsed.Path, "cgi-bin", endpointPath)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
