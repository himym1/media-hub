package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"media-hub/backend/internal/localupload"
	"media-hub/backend/internal/store"
	"net/http"
	"strconv"
	"strings"
)

func (h *handler) listLocalUploadRoots(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.LocalUploads == nil {
		writeProblem(w, problem{Title: "本地上传未配置", Status: http.StatusServiceUnavailable, Code: "not_configured"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"roots": h.dependencies.LocalUploads.Roots()})
}
func (h *handler) listLocalUploadFiles(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.LocalUploads == nil {
		writeProblem(w, problem{Title: "本地上传未配置", Status: http.StatusServiceUnavailable, Code: "not_configured"})
		return
	}
	values, err := h.dependencies.LocalUploads.List(r.URL.Query().Get("rootId"), r.URL.Query().Get("path"))
	if err != nil {
		writeLocalUploadProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": values})
}
func (h *handler) createLocalUpload(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.LocalUploads == nil {
		writeProblem(w, problem{Title: "本地上传未配置", Status: http.StatusServiceUnavailable, Code: "not_configured"})
		return
	}
	var input struct {
		RootID        string `json:"rootId"`
		Path          string `json:"path"`
		DestinationID string `json:"destinationId"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 32<<10)).Decode(&input); err != nil {
		writeLocalUploadProblem(w, localupload.ErrInvalidPath)
		return
	}
	value, err := h.dependencies.LocalUploads.Create(r.Context(), principalFromContext(r.Context()).UserID, strings.TrimSpace(r.Header.Get("Idempotency-Key")), input.RootID, input.Path, input.DestinationID)
	if err != nil {
		writeLocalUploadProblem(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, value)
}
func (h *handler) listLocalUploads(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			writeLocalUploadProblem(w, localupload.ErrInvalidPath)
			return
		}
		limit = value
	}
	values, err := h.dependencies.LocalUploads.ListJobs(r.Context(), principalFromContext(r.Context()).UserID, limit)
	if err != nil {
		writeLocalUploadProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"uploads": values})
}
func (h *handler) getLocalUpload(w http.ResponseWriter, r *http.Request) {
	value, err := h.dependencies.LocalUploads.Get(r.Context(), principalFromContext(r.Context()).UserID, r.PathValue("id"))
	if err != nil {
		writeLocalUploadProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (h *handler) retryLocalUpload(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Confirmation string `json:"confirmation"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&input); err != nil {
		writeLocalUploadProblem(w, localupload.ErrInvalidPath)
		return
	}
	value, err := h.dependencies.LocalUploads.Retry(r.Context(), principalFromContext(r.Context()).UserID, r.PathValue("id"), input.Confirmation)
	if err != nil {
		writeLocalUploadProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func writeLocalUploadProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, localupload.ErrUnavailable):
		writeProblem(w, problem{Title: "本地上传未配置", Status: http.StatusServiceUnavailable, Code: "not_configured"})
	case errors.Is(err, localupload.ErrInvalidPath):
		writeProblem(w, problem{Title: "本地上传参数或路径无效", Status: http.StatusBadRequest, Code: "invalid_upload"})
	case errors.Is(err, store.ErrIdempotencyConflict):
		writeProblem(w, problem{Title: "幂等键已绑定其他上传", Status: http.StatusConflict, Code: "idempotency_conflict"})
	case errors.Is(err, store.ErrLocalUploadNotFound):
		writeProblem(w, problem{Title: "上传任务不存在或不可重试", Status: http.StatusNotFound, Code: "upload_not_found"})
	default:
		writeProblem(w, problem{Title: "本地上传服务暂时不可用", Status: http.StatusInternalServerError, Code: "upload_unavailable"})
	}
}
