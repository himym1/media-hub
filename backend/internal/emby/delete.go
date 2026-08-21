package emby

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

type DeletePreview struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	FileCount    int    `json:"fileCount"`
	DeletesFiles bool   `json:"deletesFiles"`
	CloudKept    bool   `json:"cloudKept"`
	VersionCount int    `json:"versionCount"`
}

type deleteInfoResponse struct {
	Paths          []string `json:"Paths"`
	LocalFileCount int      `json:"LocalFileCount"`
}

type deleteAttempt struct {
	method       string
	endpointPath string
	ids          bool
}

func (c *Client) DeletePreview(ctx context.Context, itemID string) (DeletePreview, error) {
	item, configuration, err := c.visibleItem(ctx, itemID)
	if err != nil {
		return DeletePreview{}, err
	}
	ids := c.relatedVersionIDs(ctx, configuration, item)
	fileCount := 0
	for _, id := range ids {
		fileCount += deleteFileCount(c, ctx, configuration, id)
	}
	return DeletePreview{
		ID: item.ID, Name: item.Name, Type: item.Type,
		FileCount: fileCount, DeletesFiles: true, CloudKept: true, VersionCount: len(ids),
	}, nil
}

func deleteFileCount(c *Client, ctx context.Context, configuration clientConfig, itemID string) int {
	var info deleteInfoResponse
	if err := c.getJSONWithNotFound(ctx, configuration, path.Join("Items", itemID, "DeleteInfo"), deleteQuery(configuration.userID, ""), true, &info); err != nil {
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
	var primaryErr error
	deleted := 0
	for _, id := range c.relatedVersionIDs(ctx, configuration, item) {
		err := c.deleteOne(ctx, configuration, id)
		if err == nil {
			deleted++
			continue
		}
		if errors.Is(err, ErrItemNotFound) {
			continue
		}
		if id == item.ID {
			primaryErr = err
		}
	}
	if primaryErr != nil {
		return primaryErr
	}
	if deleted == 0 {
		return ErrItemNotFound
	}
	return nil
}

func (c *Client) deleteOne(ctx context.Context, configuration clientConfig, itemID string) error {
	err := c.deleteItemWithSession(ctx, configuration, itemID)
	if !errors.Is(err, ErrUnauthorized) {
		return err
	}
	c.clearSessionToken()
	return c.deleteItemWithSession(ctx, configuration, itemID)
}

func (c *Client) deleteItemWithSession(ctx context.Context, configuration clientConfig, itemID string) error {
	token, err := c.userSessionToken(ctx, configuration)
	if err != nil {
		return err
	}
	session := configuration
	session.apiKey = token
	var last error
	for _, attempt := range []deleteAttempt{
		{http.MethodDelete, path.Join("Items", itemID), false},
		{http.MethodPost, path.Join("Items", itemID, "Delete"), false},
		{http.MethodPost, "Items/Delete", true},
		{http.MethodDelete, "Items", true},
	} {
		query := deleteQuery(session.userID, "")
		if attempt.ids {
			query = deleteQuery(session.userID, itemID)
		}
		err := c.delete(ctx, session, attempt.endpointPath, query, attempt.method)
		if err == nil {
			return nil
		}
		if errors.Is(err, ErrUnauthorized) {
			return err
		}
		last = err
	}
	return last
}

func (c *Client) relatedVersionIDs(ctx context.Context, configuration clientConfig, item baseItem) []string {
	ids := []string{item.ID}
	seen := map[string]struct{}{strings.ToLower(item.ID): {}}
	for _, candidate := range c.findRelatedItems(ctx, configuration, item) {
		id := strings.TrimSpace(candidate.ID)
		if id == "" || !is115Item(candidate) || !sameLibraryVersion(item, candidate) {
			continue
		}
		key := strings.ToLower(id)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func (c *Client) findRelatedItems(ctx context.Context, configuration clientConfig, item baseItem) []baseItem {
	var items []baseItem
	for _, libraryID := range configuredLibraryIDs(configuration) {
		query := url.Values{
			"Fields":           {"ProviderIds,MediaSources,Path"},
			"IncludeItemTypes": {item.Type},
			"Limit":            {"50"},
			"ParentId":         {libraryID},
			"Recursive":        {"true"},
			"SearchTerm":       {item.Name},
		}
		if configuration.userID != "" {
			query.Set("UserId", configuration.userID)
		}
		if item.ProductionYear > 0 {
			query.Set("Years", strconv.Itoa(item.ProductionYear))
		}
		var response itemResponse
		if err := c.getJSON(ctx, configuration, "Items", query, true, &response); err != nil {
			continue
		}
		items = append(items, response.Items...)
	}
	return items
}

func sameLibraryVersion(item, candidate baseItem) bool {
	if !strings.EqualFold(item.Type, candidate.Type) {
		return false
	}
	itemTmdb := providerID(item, "Tmdb")
	candidateTmdb := providerID(candidate, "Tmdb")
	if itemTmdb != "" && candidateTmdb != "" {
		return itemTmdb == candidateTmdb
	}
	if !strings.EqualFold(strings.TrimSpace(item.Name), strings.TrimSpace(candidate.Name)) {
		return false
	}
	return item.ProductionYear == 0 || candidate.ProductionYear == 0 || item.ProductionYear == candidate.ProductionYear
}

func providerID(item baseItem, key string) string {
	for name, value := range item.ProviderIDs {
		if strings.EqualFold(name, key) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func deleteQuery(userID, itemID string) url.Values {
	query := url.Values{}
	if itemID != "" {
		query.Set("Ids", itemID)
	}
	if userID != "" {
		query.Set("UserId", userID)
	}
	return query
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
	if !strings.EqualFold(item.ID, itemID) || item.Name == "" {
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

func (c *Client) delete(ctx context.Context, configuration clientConfig, endpointPath string, query url.Values, method string) error {
	if method == "" {
		method = http.MethodDelete
	}
	endpoint, err := endpointURL(configuration.baseURL, endpointPath, query)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create Emby request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Media-Hub/emby")
	applyEmbyAuth(request, configuration)
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
	if response.StatusCode == http.StatusBadRequest {
		return ErrDeleteRejected
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ErrUpstreamResponse
	}
	return nil
}
