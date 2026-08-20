package emby

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type DeletePreview struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	FileCount    int    `json:"fileCount"`
	DeletesFiles bool   `json:"deletesFiles"`
	CloudKept    bool   `json:"cloudKept"`
}

type deleteInfoResponse struct {
	Paths          []string `json:"Paths"`
	LocalFileCount int      `json:"LocalFileCount"`
}

func (c *Client) DeletePreview(ctx context.Context, itemID string) (DeletePreview, error) {
	item, configuration, err := c.visibleItem(ctx, itemID)
	if err != nil {
		return DeletePreview{}, err
	}
	return DeletePreview{
		ID: item.ID, Name: item.Name, Type: item.Type,
		FileCount: deleteFileCount(c, ctx, configuration, item.ID), DeletesFiles: true, CloudKept: true,
	}, nil
}

func deleteFileCount(c *Client, ctx context.Context, configuration clientConfig, itemID string) int {
	query := url.Values{}
	if configuration.userID != "" {
		query.Set("UserId", configuration.userID)
	}
	var info deleteInfoResponse
	if err := c.getJSONWithNotFound(ctx, configuration, path.Join("Items", itemID, "DeleteInfo"), query, true, &info); err != nil {
		return 0
	}
	if info.LocalFileCount > len(info.Paths) {
		return info.LocalFileCount
	}
	return len(info.Paths)
}

func (c *Client) DeleteItem(ctx context.Context, itemID string) error {
	item, configuration, err := c.visibleItem(ctx, itemID)
	if err != nil {
		return err
	}
	query := url.Values{"Recursive": {"true"}}
	if err := c.delete(ctx, configuration, path.Join("Items", item.ID), query); err == nil || errors.Is(err, ErrUnauthorized) {
		return err
	}
	return c.delete(ctx, configuration, path.Join("Items", item.ID, "Delete"), query, http.MethodPost)
}

func (c *Client) visibleItem(ctx context.Context, itemID string) (baseItem, clientConfig, error) {
	configuration := c.configuration()
	if err := validateAuthenticated(configuration); err != nil {
		return baseItem{}, configuration, err
	}
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return baseItem{}, configuration, ErrUpstreamResponse
	}
	query := url.Values{"Fields": {"MediaSources,Path"}}
	endpointPath := path.Join("Items", itemID)
	if configuration.userID != "" {
		endpointPath = path.Join("Users", configuration.userID, "Items", itemID)
	}
	var item baseItem
	if err := c.getJSONWithNotFound(ctx, configuration, endpointPath, query, true, &item); err != nil {
		return baseItem{}, configuration, err
	}
	if item.ID != itemID || item.Name == "" {
		return baseItem{}, configuration, ErrItemNotFound
	}
	visible, err := c.cloudItemVisible(ctx, configuration, item)
	if err != nil {
		return baseItem{}, configuration, err
	}
	if !visible {
		return baseItem{}, configuration, ErrItemNotFound
	}
	return item, configuration, nil
}

func (c *Client) delete(ctx context.Context, configuration clientConfig, endpointPath string, query url.Values, method ...string) error {
	verb := http.MethodDelete
	if len(method) > 0 && method[0] != "" {
		verb = method[0]
	}
	endpoint, err := endpointURL(configuration.baseURL, endpointPath, query)
	if err != nil {
		return err
	}
	var body io.Reader
	if verb == http.MethodPost {
		body = strings.NewReader("{}")
	}
	request, err := http.NewRequestWithContext(ctx, verb, endpoint, body)
	if err != nil {
		return fmt.Errorf("create Emby request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Media-Hub/emby")
	request.Header.Set("X-Emby-Token", configuration.apiKey)
	if verb == http.MethodPost {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("request Emby: %w", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	if response.StatusCode == http.StatusNotFound {
		return ErrItemNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ErrUpstreamResponse
	}
	return nil
}
