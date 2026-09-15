package emby

import (
	"context"
	"net/url"
	"sort"
	"strings"
)

const (
	adultLibraryID   = "adult"
	adultLibraryName = "成人影视"
)

func isAdultLibraryID(id string) bool {
	return strings.TrimSpace(id) == adultLibraryID
}

func adultLibrary() Library {
	return Library{ID: adultLibraryID, Name: adultLibraryName, CollectionType: "movies"}
}

func isAdultLibraryName(name string) bool {
	compact := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", ""))
	compact = strings.ReplaceAll(compact, "-", "")
	compact = strings.ReplaceAll(compact, "_", "")
	for _, marker := range []string{"成人", "情色", "里番", "十八禁", "adult", "xxx", "porn", "hentai", "jav", "r18", "18+", "nc17", "nsfw"} {
		if strings.Contains(compact, marker) {
			return true
		}
	}
	return false
}

func isAdultFolder(item baseItem) bool {
	if item.ID == "" || strings.TrimSpace(item.Name) == "" || !isAdultLibraryName(item.Name) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(item.CollectionType)) {
	case "", "movies", "tvshows", "boxsets", "homevideos", "mixed":
		return true
	default:
		return false
	}
}

func isAdultItem(item baseItem) bool {
	if isAdultRating(item.OfficialRating) {
		return true
	}
	for _, genre := range item.Genres {
		if isAdultGenre(genre) {
			return true
		}
	}
	return isAdultPath(item.Path)
}

func isAdultRating(value string) bool {
	compact := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
	compact = strings.ReplaceAll(compact, "-", "")
	switch compact {
	case "xxx", "ao", "x", "r18", "r18+", "18", "18+", "nc17", "nc-17":
		return true
	default:
		return strings.Contains(compact, "xxx") || strings.Contains(compact, "r18")
	}
}

func isAdultGenre(value string) bool {
	compact := strings.ToLower(strings.TrimSpace(value))
	for _, marker := range []string{"成人", "情色", "里番", "adult", "erotica", "hentai", "xxx"} {
		if strings.Contains(compact, marker) {
			return true
		}
	}
	return false
}

func isAdultPath(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	parts := strings.FieldsFunc(trimmed, func(item rune) bool {
		return item == '/' || item == '\\'
	})
	for _, part := range parts {
		if isAdultLibraryName(part) {
			return true
		}
	}
	return false
}

func excludeAdultItems(items []baseItem) []baseItem {
	filtered := make([]baseItem, 0, len(items))
	for _, item := range items {
		if isAdultItem(item) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func (c *Client) cachedAdultFolderIDs() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return append([]string(nil), c.adultFolderIDs...)
}

func (c *Client) browseAdultItems(ctx context.Context, configuration clientConfig, offset, limit int) (SearchResult, error) {
	collected := make([]baseItem, 0, 64)
	seen := make(map[string]struct{})
	appendUnique := func(items []baseItem) {
		for _, item := range items {
			if item.ID == "" {
				continue
			}
			if _, exists := seen[item.ID]; exists {
				continue
			}
			seen[item.ID] = struct{}{}
			collected = append(collected, item)
		}
	}
	for _, folderID := range c.cachedAdultFolderIDs() {
		items, err := c.listFolderCatalog(ctx, configuration, folderID, false)
		if err != nil {
			continue
		}
		appendUnique(catalogItems(items))
	}
	tagged, err := c.listTaggedAdultItems(ctx, configuration)
	if err == nil {
		appendUnique(catalogItems(tagged))
	}
	visible := make([]baseItem, 0, len(collected))
	for _, item := range collected {
		ok, visibleErr := c.catalogItemVisible(ctx, configuration, item)
		if visibleErr != nil || !ok {
			continue
		}
		visible = append(visible, item)
	}
	sort.SliceStable(visible, func(i, j int) bool {
		return strings.ToLower(visible[i].Name) < strings.ToLower(visible[j].Name)
	})
	if offset > len(visible) {
		offset = len(visible)
	}
	end := offset + limit
	if end > len(visible) {
		end = len(visible)
	}
	return publicItems(itemResponse{Items: visible[offset:end], TotalRecordCount: len(visible)}), nil
}

func (c *Client) listFolderCatalog(ctx context.Context, configuration clientConfig, folderID string, withUser bool) ([]baseItem, error) {
	query := url.Values{
		"Fields":           {"OfficialRating,Genres,ProviderIds,UserData,MediaSources,Path"},
		"IncludeItemTypes": {"Movie,Series"},
		"Limit":            {"10000"},
		"ParentId":         {folderID},
		"Recursive":        {"true"},
		"SortBy":           {"SortName"},
		"SortOrder":        {"Ascending"},
		"StartIndex":       {"0"},
	}
	if withUser && configuration.userID != "" {
		query.Set("UserId", configuration.userID)
	}
	var response itemResponse
	if err := c.getJSON(ctx, configuration, "Items", query, true, &response); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (c *Client) listTaggedAdultItems(ctx context.Context, configuration clientConfig) ([]baseItem, error) {
	query := url.Values{
		"Fields":           {"OfficialRating,Genres,ProviderIds,UserData,MediaSources,Path"},
		"IncludeItemTypes": {"Movie,Series"},
		"Limit":            {"10000"},
		"Recursive":        {"true"},
		"SortBy":           {"SortName"},
		"SortOrder":        {"Ascending"},
		"StartIndex":       {"0"},
	}
	var response itemResponse
	if err := c.getJSON(ctx, configuration, "Items", query, true, &response); err != nil {
		return nil, err
	}
	tagged := make([]baseItem, 0)
	for _, item := range response.Items {
		if isAdultItem(item) {
			tagged = append(tagged, item)
		}
	}
	return tagged, nil
}

func (c *Client) appendAdultSearch(ctx context.Context, configuration clientConfig, queryText string, appendUnique func([]baseItem, bool)) {
	query := url.Values{
		"Fields":           {"OfficialRating,Genres,ProviderIds,MediaSources,Path"},
		"IncludeItemTypes": {"Movie,Series"},
		"Limit":            {"100"},
		"Recursive":        {"true"},
		"SearchTerm":       {queryText},
	}
	for _, folderID := range c.cachedAdultFolderIDs() {
		query.Set("ParentId", folderID)
		var response itemResponse
		if err := c.getJSON(ctx, configuration, "Items", query, true, &response); err != nil {
			continue
		}
		appendUnique(response.Items, false)
	}
	query.Del("ParentId")
	var tagged itemResponse
	if err := c.getJSON(ctx, configuration, "Items", query, true, &tagged); err != nil {
		return
	}
	adult := make([]baseItem, 0)
	for _, item := range tagged.Items {
		if isAdultItem(item) {
			adult = append(adult, item)
		}
	}
	appendUnique(adult, false)
}
