package emby

import (
	"context"
	"net/url"
	"strings"
)

func is115Item(item baseItem) bool {
	if is115Path(item.Path) {
		return true
	}
	for _, source := range item.MediaSources {
		if is115Source(source) {
			return true
		}
	}
	return false
}

func is115Source(source mediaSource) bool {
	return strings.EqualFold(strings.TrimSpace(source.Container), "strm") ||
		is115Path(source.Path) ||
		is115Path(source.DirectStreamURL)
}

func is115Path(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.HasSuffix(lower, ".strm") || strings.Contains(lower, "pickcode=") || strings.Contains(lower, "pick_code=") {
		return true
	}
	return pickCodeFromValue(trimmed) != ""
}

func pickCodeFromValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Host != "" {
		query := parsed.Query()
		if code := validPickCode(firstNonEmpty(query.Get("pickcode"), query.Get("pick_code"))); code != "" {
			return code
		}
	}
	if index := strings.IndexAny(trimmed, "\r\n"); index >= 0 {
		trimmed = strings.TrimSpace(trimmed[:index])
	}
	return validPickCode(trimmed)
}

func validPickCode(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 4 || len(value) > 32 {
		return ""
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') {
			return ""
		}
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func filter115Items(items []baseItem, allowedSeries map[string]struct{}) []baseItem {
	filtered := make([]baseItem, 0, len(items))
	for _, item := range items {
		if item.ID == "" || item.Name == "" {
			continue
		}
		switch item.Type {
		case "Movie", "Episode":
			if is115Item(item) {
				filtered = append(filtered, item)
			}
		case "Series":
			if is115Item(item) {
				filtered = append(filtered, item)
				continue
			}
			if _, ok := allowedSeries[item.ID]; ok {
				filtered = append(filtered, item)
			}
		}
	}
	return filtered
}

func (c *Client) cloudSeriesIDs(ctx context.Context, configuration clientConfig, libraryID string) (map[string]struct{}, error) {
	query := url.Values{
		"Fields":           {"MediaSources,Path,SeriesId"},
		"IncludeItemTypes": {"Episode"},
		"Limit":            {"10000"},
		"ParentId":         {libraryID},
		"Recursive":        {"true"},
		"StartIndex":       {"0"},
	}
	if configuration.userID != "" {
		query.Set("UserId", configuration.userID)
	}
	var response itemResponse
	if err := c.getJSON(ctx, configuration, "Items", query, true, &response); err != nil {
		return nil, err
	}
	ids := make(map[string]struct{})
	for _, item := range response.Items {
		if item.Type != "Episode" || item.SeriesID == "" || !is115Item(item) {
			continue
		}
		ids[item.SeriesID] = struct{}{}
	}
	return ids, nil
}

func (c *Client) cloudItemVisible(ctx context.Context, configuration clientConfig, item baseItem) (bool, error) {
	if item.Type != "Series" {
		return is115Item(item), nil
	}
	if is115Item(item) {
		return true, nil
	}
	episodes, err := c.Episodes(ctx, item.ID)
	if err != nil {
		return false, err
	}
	return len(episodes) > 0, nil
}
