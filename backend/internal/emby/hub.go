package emby

import (
	"context"
	"strings"

	"media-hub/backend/internal/integration"
	"media-hub/backend/internal/playback"
)

// Hub is the local NAS Emby catalog used for browse, play, and library writes.
type Hub struct {
	Local *Client
}

func NewHub(local *Client) *Hub {
	return &Hub{Local: local}
}

func (h *Hub) ConfigureLocal(configuration RuntimeConfig) {
	if h != nil && h.Local != nil {
		h.Local.Configure(configuration)
	}
}

func (h *Hub) Check(ctx context.Context) integration.Health {
	if h == nil || h.Local == nil {
		return integration.Health{ID: "emby", Label: "Emby", Status: integration.StatusUnconfigured, Detail: "未配置"}
	}
	return h.Local.Check(ctx)
}

func (h *Hub) Libraries(ctx context.Context) ([]Library, error) {
	if h.Local == nil || !h.Local.Configured() {
		return nil, nil
	}
	return h.Local.Libraries(ctx)
}

func (h *Hub) SearchItems(ctx context.Context, queryText string, limit int) (SearchResult, error) {
	if h.Local == nil || !h.Local.Configured() {
		return SearchResult{}, nil
	}
	return h.Local.SearchItems(ctx, strings.TrimSpace(queryText), limit)
}

func (h *Hub) BrowseItems(ctx context.Context, libraryID string, offset, limit int) (SearchResult, error) {
	if h.Local == nil {
		return SearchResult{}, ErrNotConfigured
	}
	return h.Local.BrowseItems(ctx, libraryID, offset, limit)
}

func (h *Hub) ItemDetails(ctx context.Context, itemID string) (ItemDetail, error) {
	if h.Local == nil {
		return ItemDetail{}, ErrNotConfigured
	}
	return h.Local.ItemDetails(ctx, itemID)
}

func (h *Hub) Episodes(ctx context.Context, seriesID string) ([]Episode, error) {
	if h.Local == nil {
		return nil, ErrNotConfigured
	}
	return h.Local.Episodes(ctx, seriesID)
}

func (h *Hub) PrimaryImage(ctx context.Context, itemID string, maxWidth int) (PrimaryImage, error) {
	if h.Local == nil {
		return PrimaryImage{}, ErrNotConfigured
	}
	return h.Local.PrimaryImage(ctx, itemID, maxWidth)
}

func (h *Hub) RefreshLibrary(ctx context.Context, libraryID string) error {
	if h.Local == nil {
		return ErrNotConfigured
	}
	return h.Local.RefreshLibrary(ctx, libraryID)
}

func (h *Hub) RefreshItem(ctx context.Context, itemID string) error {
	if h.Local == nil {
		return ErrNotConfigured
	}
	return h.Local.RefreshItem(ctx, itemID)
}

func (h *Hub) DeletePreview(ctx context.Context, itemID string) (DeletePreview, error) {
	if h.Local == nil {
		return DeletePreview{}, ErrNotConfigured
	}
	return h.Local.DeletePreview(ctx, itemID)
}

func (h *Hub) DeleteItem(ctx context.Context, itemID string) error {
	if h.Local == nil {
		return ErrNotConfigured
	}
	return h.Local.DeleteItem(ctx, itemID)
}

func (h *Hub) SearchRemoteSubtitles(ctx context.Context, itemID, language string) ([]RemoteSubtitle, error) {
	if h.Local == nil {
		return nil, ErrNotConfigured
	}
	return h.Local.SearchRemoteSubtitles(ctx, itemID, language)
}

func (h *Hub) DownloadRemoteSubtitle(ctx context.Context, itemID, subtitleID string) error {
	if h.Local == nil {
		return ErrNotConfigured
	}
	return h.Local.DownloadRemoteSubtitle(ctx, itemID, subtitleID)
}

func (h *Hub) ResolveEmbyItem(ctx context.Context, target playback.EmbyItemTarget, playbackUserAgent string) (playback.SourceMedia, error) {
	if h.Local == nil {
		return playback.SourceMedia{}, playback.ErrSourceNotConfigured
	}
	return h.Local.ResolveEmbyItem(ctx, target, playbackUserAgent)
}

func (h *Hub) ReportPlayback(ctx context.Context, reference string, event playback.SessionEvent) error {
	if h.Local != nil {
		return h.Local.ReportPlayback(ctx, reference, event)
	}
	return playback.ErrUnavailable
}

// Ensure Hub satisfies the playback resolver/reporter contracts used by main.
var (
	_ playback.EmbyResolver    = (*Hub)(nil)
	_ playback.SessionReporter = (*Hub)(nil)
	_ integration.Checker      = (*Hub)(nil)
)
