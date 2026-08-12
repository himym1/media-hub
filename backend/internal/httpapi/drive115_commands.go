package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"media-hub/backend/internal/drive115"
	"media-hub/backend/internal/store"
)

func (h *handler) createDrive115Command(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Drive115Commands == nil {
		writeProblem(w, problem{Title: "115 文件命令未配置", Status: http.StatusServiceUnavailable, Code: "not_configured"})
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		writeProblem(w, problem{Title: "缺少 Idempotency-Key", Status: http.StatusBadRequest, Code: "missing_idempotency_key"})
		return
	}
	var input drive115.CommandInput
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	if err := decoder.Decode(&input); err != nil {
		writeProblem(w, problem{Title: "命令参数无效", Status: http.StatusBadRequest, Code: "invalid_request"})
		return
	}
	value, err := h.dependencies.Drive115Commands.Create(r.Context(), principalFromContext(r.Context()).UserID, key, input)
	if err != nil {
		writeDrive115CommandProblem(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, value)
}
func (h *handler) listDrive115Commands(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			writeProblem(w, problem{Title: "limit 必须是 1 到 100 的整数", Status: http.StatusBadRequest, Code: "invalid_limit"})
			return
		}
		limit = value
	}
	values, err := h.dependencies.Drive115Commands.List(r.Context(), principalFromContext(r.Context()).UserID, limit)
	if err != nil {
		writeDrive115CommandProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"commands": values})
}
func (h *handler) getDrive115Command(w http.ResponseWriter, r *http.Request) {
	value, err := h.dependencies.Drive115Commands.Get(r.Context(), principalFromContext(r.Context()).UserID, r.PathValue("id"))
	if err != nil {
		writeDrive115CommandProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (h *handler) confirmDrive115Command(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Confirmation string `json:"confirmation"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&input); err != nil {
		writeProblem(w, problem{Title: "确认参数无效", Status: http.StatusBadRequest, Code: "invalid_request"})
		return
	}
	value, err := h.dependencies.Drive115Commands.Confirm(r.Context(), principalFromContext(r.Context()).UserID, r.PathValue("id"), input.Confirmation)
	if err != nil {
		writeDrive115CommandProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (h *handler) retryDrive115Command(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Confirmation string `json:"confirmation"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&input); err != nil {
		writeProblem(w, problem{Title: "确认参数无效", Status: http.StatusBadRequest, Code: "invalid_request"})
		return
	}
	value, err := h.dependencies.Drive115Commands.Retry(r.Context(), principalFromContext(r.Context()).UserID, r.PathValue("id"), input.Confirmation)
	if err != nil {
		writeDrive115CommandProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func writeDrive115CommandProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, drive115.ErrInvalidCommand):
		writeProblem(w, problem{Title: "115 命令参数无效", Status: http.StatusBadRequest, Code: "invalid_command"})
	case errors.Is(err, store.ErrIdempotencyConflict):
		writeProblem(w, problem{Title: "幂等键已绑定其他请求", Status: http.StatusConflict, Code: "idempotency_conflict"})
	case errors.Is(err, store.ErrDrive115CommandNotFound):
		writeProblem(w, problem{Title: "115 命令不存在或不可确认", Status: http.StatusNotFound, Code: "command_not_found"})
	default:
		writeProblem(w, problem{Title: "115 命令服务暂时不可用", Status: http.StatusInternalServerError, Code: "command_unavailable"})
	}
}
