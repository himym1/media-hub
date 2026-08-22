package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"media-hub/backend/internal/drive115"
	"media-hub/backend/internal/emby"
	"media-hub/backend/internal/qms"
)

var embyIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func (h *handler) getQMediaSyncStatus(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.QMediaSync == nil {
		writeIntegrationUnavailable(w)
		return
	}
	status, err := h.dependencies.QMediaSync.ReadStatus(r.Context())
	if err != nil {
		writeIntegrationProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *handler) getEmbyLibraries(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Emby == nil {
		writeIntegrationUnavailable(w)
		return
	}
	libraries, err := h.dependencies.Emby.Libraries(r.Context())
	if err != nil {
		writeIntegrationProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"libraries": libraries})
}

func (h *handler) searchEmbyItems(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" || len([]rune(query)) > 120 {
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/invalid-query",
			Title: "媒体库搜索词无效", Status: http.StatusBadRequest,
			Code: "invalid_query",
		})
		return
	}
	limit := 20
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			writeProblem(w, problem{
				Type:  "https://media-hub.local/problems/invalid-limit",
				Title: "媒体库结果数量无效", Status: http.StatusBadRequest,
				Code: "invalid_limit",
			})
			return
		}
		limit = parsed
	}
	if h.dependencies.Emby == nil {
		writeIntegrationUnavailable(w)
		return
	}
	result, err := h.dependencies.Emby.SearchItems(r.Context(), query, limit)
	if err != nil {
		writeIntegrationProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) browseEmbyLibraryItems(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Emby == nil {
		writeIntegrationUnavailable(w)
		return
	}
	libraryID := r.PathValue("id")
	if !embyIDPattern.MatchString(libraryID) {
		writeInvalidEmbyID(w)
		return
	}
	offset, limit, ok := readEmbyPage(w, r)
	if !ok {
		return
	}
	result, err := h.dependencies.Emby.BrowseItems(r.Context(), libraryID, offset, limit)
	if err != nil {
		writeIntegrationProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) getEmbyItem(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Emby == nil {
		writeIntegrationUnavailable(w)
		return
	}
	itemID := r.PathValue("id")
	if !embyIDPattern.MatchString(itemID) {
		writeInvalidEmbyID(w)
		return
	}
	item, err := h.dependencies.Emby.ItemDetails(r.Context(), itemID)
	if err != nil {
		writeIntegrationProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *handler) getEmbyEpisodes(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Emby == nil {
		writeIntegrationUnavailable(w)
		return
	}
	seriesID := r.PathValue("id")
	if !embyIDPattern.MatchString(seriesID) {
		writeInvalidEmbyID(w)
		return
	}
	episodes, err := h.dependencies.Emby.Episodes(r.Context(), seriesID)
	if err != nil {
		writeIntegrationProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": episodes, "total": len(episodes)})
}

func (h *handler) getEmbyPrimaryImage(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Emby == nil {
		writeIntegrationUnavailable(w)
		return
	}
	itemID := r.PathValue("id")
	if !embyIDPattern.MatchString(itemID) {
		writeInvalidEmbyID(w)
		return
	}
	const maxWidth = 320
	if h.dependencies.EmbyPosterCache != nil {
		if image, ok := h.dependencies.EmbyPosterCache.Get(itemID, maxWidth); ok {
			writeEmbyPrimaryImage(w, image, true)
			return
		}
	}
	image, err := h.dependencies.Emby.PrimaryImage(r.Context(), itemID, maxWidth)
	if err != nil {
		writeIntegrationProblem(w, err)
		return
	}
	if h.dependencies.EmbyPosterCache != nil {
		_ = h.dependencies.EmbyPosterCache.Put(itemID, maxWidth, image)
	}
	writeEmbyPrimaryImage(w, image, false)
}

func writeEmbyPrimaryImage(w http.ResponseWriter, image emby.PrimaryImage, cached bool) {
	w.Header().Set("Content-Type", image.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(image.Data)))
	w.Header().Set("Cache-Control", "private, max-age=604800")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if cached {
		w.Header().Set("X-Poster-Cache", "HIT")
	} else {
		w.Header().Set("X-Poster-Cache", "MISS")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(image.Data)
}

func (h *handler) refreshEmbyLibrary(w http.ResponseWriter, r *http.Request) {
	h.refreshEmbyObject(w, r, true)
}

func (h *handler) refreshEmbyItem(w http.ResponseWriter, r *http.Request) {
	h.refreshEmbyObject(w, r, false)
}

func (h *handler) previewEmbyItemDelete(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Emby == nil {
		writeIntegrationUnavailable(w)
		return
	}
	id := r.PathValue("id")
	if !embyIDPattern.MatchString(id) {
		writeInvalidEmbyID(w)
		return
	}
	preview, err := h.dependencies.Emby.DeletePreview(r.Context(), id)
	if err != nil {
		writeIntegrationProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (h *handler) deleteEmbyItem(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Emby == nil {
		writeIntegrationUnavailable(w)
		return
	}
	id := r.PathValue("id")
	if !embyIDPattern.MatchString(id) {
		writeInvalidEmbyID(w)
		return
	}
	var input struct {
		Confirmation string `json:"confirmation"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&input); err != nil || strings.TrimSpace(input.Confirmation) != id {
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/invalid-confirmation",
			Title: "删除确认无效", Status: http.StatusBadRequest,
			Code: "invalid_confirmation",
		})
		return
	}
	preview, err := h.dependencies.Emby.DeletePreview(r.Context(), id)
	if err != nil {
		writeIntegrationProblem(w, err)
		return
	}
	if err := h.dependencies.Emby.DeleteItem(r.Context(), id); err != nil {
		writeIntegrationProblem(w, err)
		return
	}
	h.notifyLibraryDelete(r.Context(), preview)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *handler) notifyLibraryDelete(ctx context.Context, preview emby.DeletePreview) {
	if h.dependencies.WeComTester == nil || !h.dependencies.WeComTester.Configured() {
		return
	}
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
	defer cancel()
	message := libraryDeleteNotification(preview)
	for attempt := 0; attempt < 3; attempt++ {
		unknown, err := h.dependencies.WeComTester.Send(sendCtx, message)
		if err == nil && !unknown {
			return
		}
		if unknown || sendCtx.Err() != nil {
			return
		}
	}
}

func libraryDeleteNotification(preview emby.DeletePreview) string {
	name := strings.TrimSpace(preview.Name)
	if name == "" {
		name = "该条目"
	}
	message := "Media Hub\n《" + name + "》已从片库删除"
	if preview.VersionCount > 1 {
		message += "（" + strconv.Itoa(preview.VersionCount) + " 个版本）"
	}
	if preview.CloudKept {
		message += "\n115 云盘文件已保留"
	}
	return message
}

func (h *handler) refreshEmbyObject(w http.ResponseWriter, r *http.Request, library bool) {
	if h.dependencies.Emby == nil {
		writeIntegrationUnavailable(w)
		return
	}
	id := r.PathValue("id")
	if !embyIDPattern.MatchString(id) {
		writeInvalidEmbyID(w)
		return
	}
	var err error
	if library {
		err = h.dependencies.Emby.RefreshLibrary(r.Context(), id)
	} else {
		err = h.dependencies.Emby.RefreshItem(r.Context(), id)
	}
	if err != nil {
		writeIntegrationProblem(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (h *handler) searchEmbyRemoteSubtitles(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Emby == nil {
		writeIntegrationUnavailable(w)
		return
	}
	itemID := r.PathValue("id")
	if !embyIDPattern.MatchString(itemID) {
		writeInvalidEmbyID(w)
		return
	}
	language := strings.TrimSpace(r.URL.Query().Get("language"))
	if language == "" {
		language = "chi"
	}
	if len(language) < 2 || len(language) > 16 {
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/invalid-language",
			Title: "字幕语言无效", Status: http.StatusBadRequest,
			Code: "invalid_language",
		})
		return
	}
	items, err := h.dependencies.Emby.SearchRemoteSubtitles(r.Context(), itemID, language)
	if err != nil {
		writeIntegrationProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *handler) downloadEmbyRemoteSubtitle(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Emby == nil {
		writeIntegrationUnavailable(w)
		return
	}
	itemID := r.PathValue("id")
	if !embyIDPattern.MatchString(itemID) {
		writeInvalidEmbyID(w)
		return
	}
	var input struct {
		SubtitleID string `json:"subtitleId"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&input); err != nil {
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/invalid-request",
			Title: "字幕下载请求无效", Status: http.StatusBadRequest,
			Code: "invalid_request",
		})
		return
	}
	subtitleID := strings.TrimSpace(input.SubtitleID)
	if subtitleID == "" || len(subtitleID) > 512 {
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/invalid-subtitle-id",
			Title: "字幕编号无效", Status: http.StatusBadRequest,
			Code: "invalid_subtitle_id",
		})
		return
	}
	if err := h.dependencies.Emby.DownloadRemoteSubtitle(r.Context(), itemID, subtitleID); err != nil {
		writeIntegrationProblem(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func readEmbyPage(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	offset := 0
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 || parsed > 1_000_000 {
			writeProblem(w, problem{Type: "https://media-hub.local/problems/invalid-offset", Title: "媒体库偏移量无效", Status: http.StatusBadRequest, Code: "invalid_offset"})
			return 0, 0, false
		}
		offset = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writeProblem(w, problem{Type: "https://media-hub.local/problems/invalid-limit", Title: "媒体库结果数量无效", Status: http.StatusBadRequest, Code: "invalid_limit"})
			return 0, 0, false
		}
		limit = parsed
	}
	return offset, limit, true
}

func writeInvalidEmbyID(w http.ResponseWriter) {
	writeProblem(w, problem{
		Type: "https://media-hub.local/problems/invalid-emby-id", Title: "Emby 对象标识无效",
		Status: http.StatusBadRequest, Code: "invalid_emby_id",
	})
}

func (h *handler) getDrive115Status(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Drive115 == nil {
		writeIntegrationUnavailable(w)
		return
	}
	status, err := h.dependencies.Drive115.Status(r.Context())
	if err != nil {
		writeIntegrationProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func writeIntegrationProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, emby.ErrItemNotFound):
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/emby-item-not-found",
			Title: "Emby 媒体不存在", Status: http.StatusNotFound, Code: "emby_item_not_found",
		})
	case errors.Is(err, context.DeadlineExceeded):
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/integration-timeout",
			Title: "外部服务响应超时", Status: http.StatusGatewayTimeout,
			Code: "integration_timeout",
		})
	case errors.Is(err, qms.ErrNotConfigured),
		errors.Is(err, qms.ErrMissingAPIKey),
		errors.Is(err, emby.ErrNotConfigured),
		errors.Is(err, emby.ErrMissingAPIKey),
		errors.Is(err, drive115.ErrNotConfigured):
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/integration-incomplete",
			Title: "集成配置不完整", Status: http.StatusServiceUnavailable,
			Code: "integration_incomplete",
		})
	case errors.Is(err, qms.ErrUnauthorized),
		errors.Is(err, emby.ErrUnauthorized),
		errors.Is(err, drive115.ErrUnauthorized):
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/integration-unauthorized",
			Title: "外部服务鉴权失败", Status: http.StatusBadGateway,
			Code: "integration_unauthorized",
		})
	case errors.Is(err, emby.ErrDeleteNeedsUser):
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/emby-delete-needs-user",
			Title: "Emby 删除需要用户密码", Status: http.StatusConflict,
			Code: "emby_delete_needs_user",
		})
	case errors.Is(err, emby.ErrDeleteRejected):
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/emby-delete-rejected",
			Title: "Emby 拒绝删除该媒体", Status: http.StatusBadGateway,
			Code: "emby_delete_rejected",
		})
	default:
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/integration-unavailable",
			Title: "外部服务暂时不可用", Status: http.StatusBadGateway,
			Code: "integration_unavailable",
		})
	}
}

func writeIntegrationUnavailable(w http.ResponseWriter) {
	writeProblem(w, problem{
		Type:  "https://media-hub.local/problems/integration-unavailable",
		Title: "集成服务不可用", Status: http.StatusServiceUnavailable,
		Code: "integration_unavailable",
	})
}
