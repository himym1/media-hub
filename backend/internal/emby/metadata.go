package emby

import (
	"context"
	"net/url"
	"path"
	"strings"
)

type remoteSearchQuery struct {
	SearchInfo remoteSearchInfo `json:"SearchInfo"`
	ItemID     string           `json:"ItemId"`
}

type remoteSearchInfo struct {
	Name        string            `json:"Name,omitempty"`
	Year        int               `json:"Year,omitempty"`
	ProviderIDs map[string]string `json:"ProviderIds"`
}

type remoteSearchResult struct {
	Name               string            `json:"Name"`
	ProductionYear     int               `json:"ProductionYear,omitempty"`
	ProviderIDs        map[string]string `json:"ProviderIds"`
	ImageURL           string            `json:"ImageUrl,omitempty"`
	SearchProviderName string            `json:"SearchProviderName,omitempty"`
	Overview           string            `json:"Overview,omitempty"`
}

// NeedsTMDBIdentify reports whether an indexed item still needs TMDB name/poster.
func NeedsTMDBIdentify(item Item, tmdbID string) bool {
	tmdbID = strings.TrimSpace(tmdbID)
	if tmdbID == "" || strings.TrimSpace(item.ID) == "" {
		return false
	}
	if providerTMDB(item.ProviderIDs) != tmdbID {
		return true
	}
	return LooksLikeUnidentifiedName(item.Name)
}

// LooksLikeUnidentifiedName reports release-group or site-dump titles that Emby
// could not replace with a catalog name.
func LooksLikeUnidentifiedName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	return looksLikeReleaseName(name) || looksLikeSiteDumpName(name)
}

func looksLikeSiteDumpName(name string) bool {
	compact := strings.ToLower(name)
	if strings.Contains(compact, "www.") || strings.Contains(compact, "http://") || strings.Contains(compact, "https://") {
		return true
	}
	if strings.Contains(name, "发布") || strings.Contains(name, "帧率") || strings.Contains(name, "高码") {
		return true
	}
	return strings.Contains(name, "【") && strings.Contains(name, "】")
}

func providerTMDB(ids map[string]string) string {
	if len(ids) == 0 {
		return ""
	}
	for _, key := range []string{"Tmdb", "TmdbId", "TheMovieDb", "tmdb"} {
		if value := strings.TrimSpace(ids[key]); value != "" {
			return value
		}
	}
	return ""
}

func remoteSearchPath(mediaType string) string {
	if strings.EqualFold(strings.TrimSpace(mediaType), "series") {
		return path.Join("Items", "RemoteSearch", "Series")
	}
	return path.Join("Items", "RemoteSearch", "Movie")
}

// ApplyTMDBMetadata identifies an Emby item with a known TMDB id and refreshes
// metadata/images. Prefer remote search + apply, then fall back to writing
// ProviderIds and a full refresh when folder or file names are non-standard.
func (c *Client) ApplyTMDBMetadata(ctx context.Context, itemID, mediaType, title string, year int, tmdbID string, replaceAllImages bool) error {
	configuration := c.configuration()
	if err := validateAuthenticated(configuration); err != nil {
		return err
	}
	itemID = strings.TrimSpace(itemID)
	tmdbID = strings.TrimSpace(tmdbID)
	title = strings.TrimSpace(title)
	if itemID == "" || tmdbID == "" {
		return ErrUpstreamResponse
	}
	if err := c.applyRemoteTMDBMetadata(ctx, configuration, itemID, mediaType, title, year, tmdbID, replaceAllImages); err == nil {
		return nil
	}
	return c.applyLocalTMDBMetadata(ctx, configuration, itemID, title, year, tmdbID, replaceAllImages)
}

func (c *Client) applyRemoteTMDBMetadata(
	ctx context.Context,
	configuration clientConfig,
	itemID, mediaType, title string,
	year int,
	tmdbID string,
	replaceAllImages bool,
) error {
	var results []remoteSearchResult
	if err := c.postJSONBody(ctx, configuration, remoteSearchPath(mediaType), nil, remoteSearchQuery{
		ItemID: itemID,
		SearchInfo: remoteSearchInfo{
			Name:        title,
			Year:        year,
			ProviderIDs: map[string]string{"Tmdb": tmdbID},
		},
	}, &results, false); err != nil {
		return err
	}
	chosen, ok := pickRemoteSearchResult(results, tmdbID)
	if !ok {
		return ErrUpstreamResponse
	}
	return c.postJSONBody(
		ctx, configuration, path.Join("Items", "RemoteSearch", "Apply", itemID),
		replaceImageQuery(replaceAllImages), chosen, nil, false,
	)
}

func (c *Client) applyLocalTMDBMetadata(
	ctx context.Context,
	configuration clientConfig,
	itemID, title string,
	year int,
	tmdbID string,
	replaceAllImages bool,
) error {
	raw, err := c.readItemObject(ctx, configuration, itemID)
	if err != nil {
		return err
	}
	if title != "" {
		raw["Name"] = title
	}
	if year > 0 {
		raw["ProductionYear"] = year
	}
	providers, _ := raw["ProviderIds"].(map[string]any)
	if providers == nil {
		providers = map[string]any{}
	}
	providers["Tmdb"] = tmdbID
	raw["ProviderIds"] = providers
	if err := c.postJSONBody(ctx, configuration, path.Join("Items", itemID), nil, raw, nil, false); err != nil {
		return err
	}
	query := replaceImageQuery(replaceAllImages)
	query.Set("Recursive", "true")
	query.Set("MetadataRefreshMode", "FullRefresh")
	query.Set("ImageRefreshMode", "FullRefresh")
	query.Set("ReplaceAllMetadata", "true")
	return c.postJSON(ctx, configuration, path.Join("Items", itemID, "Refresh"), query, nil)
}

func pickRemoteSearchResult(results []remoteSearchResult, tmdbID string) (remoteSearchResult, bool) {
	for _, result := range results {
		if providerTMDB(result.ProviderIDs) == tmdbID {
			return result, true
		}
	}
	if len(results) == 1 && strings.TrimSpace(results[0].Name) != "" {
		return results[0], true
	}
	return remoteSearchResult{}, false
}

func replaceImageQuery(replaceAllImages bool) url.Values {
	query := url.Values{}
	if replaceAllImages {
		query.Set("ReplaceAllImages", "true")
	}
	return query
}
