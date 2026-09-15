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
		views = nil
	}
	folders, _ := c.readEmbyMediaFolders(ctx, configuration)
	libraries, adultSources := mergeLocalLibraries(configuration, views, folders)
	adultIDs := make([]string, 0, len(adultSources))
	for _, source := range adultSources {
		adultIDs = append(adultIDs, source.ID)
	}
	libraries = append(libraries, c.collectAdultGroups(ctx, configuration, adultSources)...)
	c.storeLocalLibraries(libraries, adultIDs)
	return libraries
}

func (c *Client) readEmbyMediaFolders(ctx context.Context, configuration clientConfig) ([]baseItem, error) {
	var response itemResponse
	if err := c.getJSON(ctx, configuration, "Library/MediaFolders", nil, true, &response); err != nil {
		return nil, err
	}
	return response.Items, nil
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

func (c *Client) storeLocalLibraries(libraries []Library, adultIDs []string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.localLibraries = append([]Library(nil), libraries...)
	c.adultFolderIDs = append([]string(nil), adultIDs...)
	c.localLibrariesAt = time.Now()
}

func (c *Client) clearLocalLibraries() {
	c.localLibraries = nil
	c.adultFolderIDs = nil
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

func mergeLocalLibraries(configuration clientConfig, views, folders []baseItem) ([]Library, []Library) {
	libraries := configuredLibraries(configuration)
	seen := make(map[string]struct{}, len(libraries)+len(views)+len(folders))
	adultSeen := make(map[string]struct{})
	adultSources := make([]Library, 0)
	collectAdult := func(item baseItem) {
		if !isAdultFolder(item) {
			return
		}
		if _, exists := adultSeen[item.ID]; exists {
			return
		}
		adultSeen[item.ID] = struct{}{}
		adultSources = append(adultSources, Library{
			ID:             item.ID,
			Name:           item.Name,
			CollectionType: strings.TrimSpace(item.CollectionType),
		})
	}
	for _, library := range libraries {
		seen[library.ID] = struct{}{}
	}
	for _, item := range views {
		collectAdult(item)
		if isAdultFolder(item) || !isBrowsableLocalView(item) {
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
	for _, item := range folders {
		collectAdult(item)
	}
	libraries = append(libraries, adultLibrary())
	return libraries, adultSources
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
	return catalogItemsOfTypes(items, "Movie", "Series")
}

func catalogAdultItems(items []baseItem) []baseItem {
	return catalogItemsOfTypes(items, "Movie", "Series", "Video")
}

func catalogItemsOfTypes(items []baseItem, allowed ...string) []baseItem {
	ok := make(map[string]struct{}, len(allowed))
	for _, itemType := range allowed {
		ok[itemType] = struct{}{}
	}
	filtered := make([]baseItem, 0, len(items))
	for _, item := range items {
		if item.ID == "" || item.Name == "" {
			continue
		}
		if _, allowedType := ok[item.Type]; !allowedType {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func (c *Client) itemInLibraries(ctx context.Context, configuration clientConfig, item baseItem, libraryIDs []string) (bool, error) {
	allowed := make(map[string]struct{}, len(libraryIDs))
	for _, id := range libraryIDs {
		if id = strings.TrimSpace(id); id != "" {
			allowed[id] = struct{}{}
		}
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
