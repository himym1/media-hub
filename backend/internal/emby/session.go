package emby

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
)

type embyUserResponse struct {
	ID   string `json:"Id"`
	Name string `json:"Name"`
}

type authenticateResponse struct {
	AccessToken string `json:"AccessToken"`
}

func (c *Client) userSessionToken(ctx context.Context, configuration clientConfig) (string, error) {
	if token := c.cachedSessionToken(); token != "" {
		return token, nil
	}
	if configuration.userID == "" {
		return "", ErrDeleteNeedsUser
	}
	token, err := c.authenticateUser(ctx, configuration)
	if err != nil {
		return "", err
	}
	c.mutex.Lock()
	c.sessionToken = token
	c.mutex.Unlock()
	return token, nil
}

func (c *Client) cachedSessionToken() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.sessionToken
}

func (c *Client) clearSessionToken() {
	c.mutex.Lock()
	c.sessionToken = ""
	c.mutex.Unlock()
}

func (c *Client) authenticateUser(ctx context.Context, configuration clientConfig) (string, error) {
	token, err := c.authenticateWithPassword(ctx, configuration, path.Join("Users", configuration.userID, "Authenticate"), map[string]string{
		"Pw": configuration.password,
	})
	if err == nil {
		return token, nil
	}
	if !errors.Is(err, ErrUnauthorized) && !errors.Is(err, ErrUpstreamResponse) && !errors.Is(err, ErrItemNotFound) {
		return "", err
	}
	userName, nameErr := c.userName(ctx, configuration)
	if nameErr != nil {
		return "", mapDeleteAuthError(nameErr, configuration.password)
	}
	token, err = c.authenticateWithPassword(ctx, configuration, "Users/AuthenticateByName", map[string]string{
		"Username": userName,
		"Pw":       configuration.password,
	})
	if err != nil {
		return "", mapDeleteAuthError(err, configuration.password)
	}
	return token, nil
}

func (c *Client) userName(ctx context.Context, configuration clientConfig) (string, error) {
	var user embyUserResponse
	if err := c.getJSONWithNotFound(ctx, configuration, path.Join("Users", configuration.userID), nil, true, &user); err != nil {
		return "", err
	}
	if !strings.EqualFold(user.ID, configuration.userID) || strings.TrimSpace(user.Name) == "" {
		return "", ErrItemNotFound
	}
	return strings.TrimSpace(user.Name), nil
}

func (c *Client) authenticateWithPassword(ctx context.Context, configuration clientConfig, endpointPath string, body map[string]string) (string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("encode Emby authentication: %w", err)
	}
	var response authenticateResponse
	if err := c.postJSONPayload(ctx, configuration.baseURL, endpointPath, payload, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.AccessToken) == "" {
		return "", ErrDeleteNeedsUser
	}
	return strings.TrimSpace(response.AccessToken), nil
}

func mapDeleteAuthError(err error, password string) error {
	if errors.Is(err, ErrUnauthorized) && password != "" {
		return ErrUnauthorized
	}
	if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrUpstreamResponse) || errors.Is(err, ErrItemNotFound) {
		return ErrDeleteNeedsUser
	}
	return err
}

func (c *Client) postJSONPayload(ctx context.Context, baseURL, endpointPath string, body []byte, target any) error {
	endpoint, err := endpointURL(baseURL, endpointPath, nil)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create Emby request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Media-Hub/emby")
	applyEmbyClientAuth(request)
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("request Emby: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return ErrUnauthorized
	}
	if response.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return ErrItemNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return ErrUpstreamResponse
	}
	return decodeEmbyBody(response, target)
}
