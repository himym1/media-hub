package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"media-hub/backend/internal/drive115"
	"media-hub/backend/internal/emby"
	"media-hub/backend/internal/qms"
)

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
