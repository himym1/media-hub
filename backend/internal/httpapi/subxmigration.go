package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"media-hub/backend/internal/store"
	"media-hub/backend/internal/subx"
	"media-hub/backend/internal/subxmigration"
)

func (h *handler) getSubXMigrationReadiness(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Migration == nil {
		writeMigrationProblem(w, subxmigration.ErrUnavailable)
		return
	}
	principal := principalFromContext(r.Context())
	result, err := h.dependencies.Migration.Readiness(r.Context(), principal.UserID)
	if err != nil {
		writeMigrationProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) importSubXSubscriptions(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Migration == nil {
		writeMigrationProblem(w, subxmigration.ErrUnavailable)
		return
	}
	principal := principalFromContext(r.Context())
	result, err := h.dependencies.Migration.ImportSubscriptions(r.Context(), principal.UserID)
	if err != nil {
		writeMigrationProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) listSubXMigrationCommands(w http.ResponseWriter, r *http.Request) {
	items, err := h.dependencies.Migration.BlockingCommands(r.Context(), principalFromContext(r.Context()).UserID)
	if err != nil {
		writeMigrationProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"commands": items})
}

func (h *handler) retrySubXMigrationCommand(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Confirmation string `json:"confirmation"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&input); err != nil {
		writeMigrationProblem(w, subx.ErrInvalidInvocation)
		return
	}
	value, err := h.dependencies.Migration.RetryBlockingCommand(r.Context(), principalFromContext(r.Context()).UserID, r.PathValue("id"), input.Confirmation)
	if err != nil {
		writeMigrationProblem(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, value)
}

func writeMigrationProblem(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "migration_failed"
	message := "SubX 迁移失败"
	switch {
	case errors.Is(err, subx.ErrInvalidInvocation):
		status = http.StatusBadRequest
		code = "invalid_confirmation"
		message = "确认值必须与 source command ID 完全一致"
	case errors.Is(err, store.ErrSubXCommandNotFound):
		status = http.StatusNotFound
		code = "source_command_not_found"
		message = "SubX source command 不存在或不可重试"
	case errors.Is(err, store.ErrSubXCommandNotRetryable):
		status = http.StatusConflict
		code = "source_command_not_retryable"
		message = "SubX source command 当前不可重试"
	case errors.Is(err, subxmigration.ErrUnavailable):
		status = http.StatusServiceUnavailable
		code = "migration_unavailable"
		message = "SubX 迁移服务未配置"
	case errors.Is(err, subxmigration.ErrNoContent):
		status = http.StatusUnprocessableEntity
		code = "no_importable_subscriptions"
		message = "SubX 备份中没有可迁移的结构化订阅"
	}
	writeProblem(w, problem{Title: message, Status: status, Code: code})
}
