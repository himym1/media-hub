package emby

import (
	"context"
	"strings"

	"media-hub/backend/internal/integration"
	"media-hub/backend/internal/playback"
)

// Hub merges the NAS Emby client with an optional shared/password Emby catalog.
type Hub struct {
	Local  *Client
	Shared *Client
}

func NewHub(local, shared *Client) *Hub {
	return &Hub{Local: local, Shared: shared}
}

func (h *Hub) ConfigureLocal(configuration RuntimeConfig) {
	if h != nil && h.Local != nil {
		h.Local.Configure(configuration)
	}
}

func (h *Hub) ConfigureShared(configuration RuntimeConfig) {
	if h != nil && h.Shared != nil {
		configuration.Shared = true
		h.Shared.Configure(configuration)
	}
}

func (h *Hub) Check(ctx context.Context) integration.Health {
	if h == nil || h.Local == nil {
		return integration.Health{ID: "emby", Label: "Emby", Status: integration.StatusUnconfigured, Detail: "未配置"}
	}
	return h.Local.Check(ctx)
}

func (h *Hub) CheckShared(ctx context.Context) integration.Health {
	if h == nil || h.Shared == nil {
		return integration.Health{ID: "shared-emby", Label: "共享 Emby", Status: integration.StatusUnconfigured, Detail: "未配置"}
	}
	return h.Shared.CheckShared(ctx)
}

func (h *Hub) Libraries(ctx context.Context) ([]Library, error) {
	libraries := make([]Library, 0, 16)
	if h.Local != nil && h.Local.Configured() {
		local, err := h.Local.Libraries(ctx)
		if err != nil {
			return nil, err
		}
		libraries = append(libraries, local...)
	}
	if h.Shared != nil && h.Shared.SharedConfigured() {
		shared, err := h.Shared.sharedLibraries(ctx)
		if err != nil {
			// Shared catalog is optional: keep NAS libraries usable when the remote host is down.
			if len(libraries) == 0 {
				return nil, err
			}
			return libraries, nil
		}
		libraries = append(libraries, shared...)
	}
	return libraries, nil
}

func (h *Hub) SearchItems(ctx context.Context, queryText string, limit int) (SearchResult, error) {
	queryText = strings.TrimSpace(queryText)
	merged := SearchResult{Items: make([]Item, 0, limit)}
	seen := make(map[string]struct{})
	appendUnique := func(items []Item) {
		for _, item := range items {
			if item.ID == "" {
				continue
			}
			if _, exists := seen[item.ID]; exists {
				continue
			}
			seen[item.ID] = struct{}{}
			merged.Items = append(merged.Items, item)
		}
	}
	if h.Local != nil && h.Local.Configured() {
		local, err := h.Local.SearchItems(ctx, queryText, limit)
		if err != nil {
			return SearchResult{}, err
		}
		appendUnique(local.Items)
	}
	if h.Shared != nil && h.Shared.SharedConfigured() {
		shared, err := h.Shared.sharedSearchItems(ctx, queryText, limit)
		if err == nil {
			appendUnique(shared.Items)
		}
	}
	if len(merged.Items) > limit && limit > 0 {
		merged.Items = merged.Items[:limit]
	}
	merged.Total = len(merged.Items)
	return merged, nil
}

func (h *Hub) BrowseItems(ctx context.Context, libraryID string, offset, limit int, sort string) (SearchResult, error) {
	if IsSharedID(libraryID) {
		if h.Shared == nil {
			return SearchResult{}, ErrNotConfigured
		}
		return h.Shared.sharedBrowseItems(ctx, libraryID, offset, limit)
	}
	if h.Local == nil {
		return SearchResult{}, ErrNotConfigured
	}
	return h.Local.BrowseItems(ctx, libraryID, offset, limit, sort)
}

func (h *Hub) ItemDetails(ctx context.Context, itemID string) (ItemDetail, error) {
	if IsSharedID(itemID) {
		if h.Shared == nil {
			return ItemDetail{}, ErrNotConfigured
		}
		return h.Shared.sharedItemDetails(ctx, itemID)
	}
	if h.Local == nil {
		return ItemDetail{}, ErrNotConfigured
	}
	return h.Local.ItemDetails(ctx, itemID)
}

func (h *Hub) Episodes(ctx context.Context, seriesID string) ([]Episode, error) {
	if IsSharedID(seriesID) {
		if h.Shared == nil {
			return nil, ErrNotConfigured
		}
		return h.Shared.sharedEpisodes(ctx, seriesID)
	}
	if h.Local == nil {
		return nil, ErrNotConfigured
	}
	return h.Local.Episodes(ctx, seriesID)
}

func (h *Hub) PrimaryImage(ctx context.Context, itemID string, maxWidth int) (PrimaryImage, error) {
	if IsSharedID(itemID) {
		if h.Shared == nil {
			return PrimaryImage{}, ErrNotConfigured
		}
		return h.Shared.sharedPrimaryImage(ctx, itemID, maxWidth)
	}
	if h.Local == nil {
		return PrimaryImage{}, ErrNotConfigured
	}
	return h.Local.PrimaryImage(ctx, itemID, maxWidth)
}

func (h *Hub) BackdropImage(ctx context.Context, itemID string, maxWidth int) (PrimaryImage, error) {
	if IsSharedID(itemID) {
		if h.Shared == nil {
			return PrimaryImage{}, ErrNotConfigured
		}
		return h.Shared.sharedBackdropImage(ctx, itemID, maxWidth)
	}
	if h.Local == nil {
		return PrimaryImage{}, ErrNotConfigured
	}
	return h.Local.BackdropImage(ctx, itemID, maxWidth)
}

func (h *Hub) RefreshLibrary(ctx context.Context, libraryID string) error {
	if IsSharedID(libraryID) {
		return ErrSharedReadOnly
	}
	return h.Local.RefreshLibrary(ctx, libraryID)
}

func (h *Hub) RefreshItem(ctx context.Context, itemID string) error {
	if IsSharedID(itemID) {
		return ErrSharedReadOnly
	}
	return h.Local.RefreshItem(ctx, itemID)
}

func (h *Hub) DeletePreview(ctx context.Context, itemID string) (DeletePreview, error) {
	if IsSharedID(itemID) {
		return DeletePreview{}, ErrSharedReadOnly
	}
	return h.Local.DeletePreview(ctx, itemID)
}

func (h *Hub) DeleteItem(ctx context.Context, itemID string) error {
	if IsSharedID(itemID) {
		return ErrSharedReadOnly
	}
	return h.Local.DeleteItem(ctx, itemID)
}

func (h *Hub) SearchRemoteSubtitles(ctx context.Context, itemID, language string) ([]RemoteSubtitle, error) {
	if IsSharedID(itemID) {
		return nil, ErrSharedReadOnly
	}
	return h.Local.SearchRemoteSubtitles(ctx, itemID, language)
}

func (h *Hub) DownloadRemoteSubtitle(ctx context.Context, itemID, subtitleID string) error {
	if IsSharedID(itemID) {
		return ErrSharedReadOnly
	}
	return h.Local.DownloadRemoteSubtitle(ctx, itemID, subtitleID)
}

func (h *Hub) LocalStreamURL(ref playback.LocalRef) (string, error) {
	if IsSharedID(ref.ItemID) {
		return "", playback.ErrUnavailable
	}
	if h.Local == nil {
		return "", playback.ErrSourceNotConfigured
	}
	return h.Local.LocalStreamURL(ref)
}

func (h *Hub) ResolveEmbyItem(ctx context.Context, target playback.EmbyItemTarget, playbackUserAgent string) (playback.SourceMedia, error) {
	if IsSharedID(target.ItemID) {
		if h.Shared == nil {
			return playback.SourceMedia{}, playback.ErrSourceNotConfigured
		}
		return h.Shared.ResolveSharedItem(ctx, target, playbackUserAgent)
	}
	if h.Local == nil {
		return playback.SourceMedia{}, playback.ErrSourceNotConfigured
	}
	return h.Local.ResolveEmbyItem(ctx, target, playbackUserAgent)
}

func (h *Hub) ReportPlayback(ctx context.Context, reference string, event playback.SessionEvent) error {
	if h.Shared != nil && strings.Contains(reference, `"playSessionId":"shared-`) {
		return h.Shared.ReportPlayback(ctx, reference, event)
	}
	if h.Local != nil {
		return h.Local.ReportPlayback(ctx, reference, event)
	}
	return playback.ErrUnavailable
}

type sharedHealthChecker struct{ hub *Hub }

func (h *Hub) SharedHealthChecker() integration.Checker {
	return sharedHealthChecker{hub: h}
}

func (c sharedHealthChecker) Check(ctx context.Context) integration.Health {
	if c.hub == nil {
		return integration.Health{ID: "shared-emby", Label: "共享 Emby", Status: integration.StatusUnconfigured, Detail: "未配置"}
	}
	return c.hub.CheckShared(ctx)
}

var (
	_ playback.EmbyResolver    = (*Hub)(nil)
	_ playback.SessionReporter = (*Hub)(nil)
	_ integration.Checker      = (*Hub)(nil)
	_ integration.Checker      = sharedHealthChecker{}
)
