package emby

import (
	"context"
	"errors"
	"path"
	"strings"
	"time"
)

const localLibraryCacheTTL = 30 * time.Second

func (c *Client) Libraries(ctx context.Context) ([]Library, error) {
	configuration := c.configuration()
	if err := validateAuthenticated(configuration); err != nil {
		return nil, err
	}
	return c.listLocalLibraries(ctx, configuration), nil
}

func (c *Client) listLocalLibraries(ctx context.Context, configuration clientConfig) []Library {
	if libraries, ok := c.cachedLocalLibraries(); ok {
		return libraries
	}
	views, err := c.readEmbyViews(ctx, configuration)
	if err != nil {
		libraries := configuredLibraries(configuration)
		c.storeLocalLibraries(libraries)
		return libraries
	}
	libraries := mergeLocalLibraries(configuration, views)
	c.storeLocalLibraries(libraries)
	return libraries
}

func (c *Client) readEmbyViews(ctx context.Context, configuration clientConfig) ([]baseItem, error) {
	endpointPath := "Library/MediaFolders"
	if configuration.userID != "" {
		endpointPath = path.Join("Users", configuration.userID, "Views")
	}
	var response itemResponse
	if err := c.getJSON(ctx, configuration, endpointPath, nil, true, &response); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (c *Client) cachedLocalLibraries() ([]Library, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	if c.localLibrariesAt.IsZero() || time.Since(c.localLibrariesAt) > localLibraryCacheTTL {
		return nil, false
	}
	return append([]Library(nil), c.localLibraries...), true
}

func (c *Client) storeLocalLibraries(libraries []Library) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.localLibraries = append([]Library(nil), libraries...)
	c.localLibrariesAt = time.Now()
}

func (c *Client) clearLocalLibraries() {
	c.localLibraries = nil
	c.localLibrariesAt = time.Time{}
}

func configuredLibraries(configuration clientConfig) []Library {
	libraries := make([]Library, 0, 2)
	if configuration.movieLibraryID != "" {
		libraries = append(libraries, Library{
			ID: configuration.movieLibraryID, Name: "115电影", CollectionType: "movies",
		})
	}
	if configuration.seriesLibraryID != "" && configuration.seriesLibraryID != configuration.movieLibraryID {
		libraries = append(libraries, Library{
			ID: configuration.seriesLibraryID, Name: "115电视剧", CollectionType: "tvshows",
		})
	}
	return libraries
}

func mergeLocalLibraries(configuration clientConfig, views []baseItem) []Library {
	libraries := configuredLibraries(configuration)
	seen := make(map[string]struct{}, len(libraries)+len(views))
	for _, library := range libraries {
		seen[library.ID] = struct{}{}
	}
	for _, item := range views {
		if !isBrowsableLocalView(item) {
			continue
		}
		if _, exists := seen[item.ID]; exists {
			continue
		}
		seen[item.ID] = struct{}{}
		libraries = append(libraries, Library{
			ID:             item.ID,
			Name:           item.Name,
			CollectionType: strings.TrimSpace(item.CollectionType),
		})
	}
	return libraries
}

func isBrowsableLocalView(item baseItem) bool {
	if item.ID == "" || strings.TrimSpace(item.Name) == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(item.CollectionType)) {
	case "", "movies", "tvshows", "boxsets":
	default:
		return false
	}
	return !isIgnoredLocalLibraryName(item.Name)
}

func isIgnoredLocalLibraryName(name string) bool {
	compact := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", ""))
	for _, junk := range []string{"metube", "easyvdl", "mediago", "bililive", "livetv"} {
		if strings.Contains(compact, junk) {
			return true
		}
	}
	return strings.Contains(name, "电视直播")
}

func isConfiguredLibrary(configuration clientConfig, libraryID string) bool {
	libraryID = strings.TrimSpace(libraryID)
	if libraryID == "" {
		return false
	}
	return libraryID == configuration.movieLibraryID || libraryID == configuration.seriesLibraryID
}

func libraryAllowed(libraries []Library, libraryID string) bool {
	libraryID = strings.TrimSpace(libraryID)
	if libraryID == "" {
		return false
	}
	for _, library := range libraries {
		if library.ID == libraryID {
			return true
		}
	}
	return false
}

func catalogItems(items []baseItem) []baseItem {
	filtered := make([]baseItem, 0, len(items))
	for _, item := range items {
		if item.ID == "" || item.Name == "" {
			continue
		}
		switch item.Type {
		case "Movie", "Series":
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (c *Client) itemInNon115Library(ctx context.Context, configuration clientConfig, item baseItem) (bool, error) {
	allowed := make(map[string]struct{})
	for _, library := range c.listLocalLibraries(ctx, configuration) {
		if isConfiguredLibrary(configuration, library.ID) {
			continue
		}
		allowed[library.ID] = struct{}{}
	}
	if len(allowed) == 0 {
		return false, nil
	}
	if _, ok := allowed[strings.TrimSpace(item.ParentID)]; ok {
		return true, nil
	}
	ancestors, err := c.itemAncestorIDs(ctx, configuration, item.ID)
	if err != nil {
		if errors.Is(err, ErrItemNotFound) {
			return false, nil
		}
		return false, err
	}
	for _, ancestorID := range ancestors {
		if _, ok := allowed[ancestorID]; ok {
			return true, nil
		}
	}
	return false, nil
}

func (c *Client) itemAncestorIDs(ctx context.Context, configuration clientConfig, itemID string) ([]string, error) {
	var ancestors []baseItem
	if err := c.getJSONWithNotFound(ctx, configuration, path.Join("Items", itemID, "Ancestors"), nil, true, &ancestors); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(ancestors))
	for _, ancestor := range ancestors {
		if ancestor.ID == "" {
			continue
		}
		ids = append(ids, ancestor.ID)
	}
	return ids, nil
}
