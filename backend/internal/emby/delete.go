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
	fileCount := 0
	var info deleteInfoResponse
	if err := c.getJSONWithNotFound(ctx, configuration, path.Join("Items", item.ID, "DeleteInfo"), nil, true, &info); err != nil {
		if !isItemNotFound(err) {
			return DeletePreview{}, err
		}
	} else {
		fileCount = info.LocalFileCount
		if fileCount < len(info.Paths) {
			fileCount = len(info.Paths)
		}
	}
	return DeletePreview{
		ID: item.ID, Name: item.Name, Type: item.Type,
		FileCount: fileCount, DeletesFiles: true, CloudKept: true,
	}, nil
}

func (c *Client) DeleteItem(ctx context.Context, itemID string) error {
	item, configuration, err := c.visibleItem(ctx, itemID)
	if err != nil {
		return err
	}
	query := url.Values{"Recursive": {"true"}}
	return c.delete(ctx, configuration, path.Join("Items", item.ID), query)
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

func (c *Client) delete(ctx context.Context, configuration clientConfig, endpointPath string, query url.Values) error {
	endpoint, err := endpointURL(configuration.baseURL, endpointPath, query)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create Emby request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Media-Hub/emby")
	request.Header.Set("X-Emby-Token", configuration.apiKey)
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

func isItemNotFound(err error) bool {
	return errors.Is(err, ErrItemNotFound)
}
