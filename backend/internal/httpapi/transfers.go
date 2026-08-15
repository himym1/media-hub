package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"media-hub/backend/internal/store"
	"media-hub/backend/internal/workflow"
)

type createTransferRequest struct {
	TransferToken string `json:"transferToken"`
}

type transferListResponse struct {
	Transfers []workflow.Job `json:"transfers"`
}

type setTransferArchivedRequest struct {
	Archived *bool `json:"archived"`
}

func (h *handler) createTransfer(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Workflow == nil {
		writeTransferProblem(w, workflow.ErrUnavailable)
		return
	}
	var request createTransferRequest
	if err := decodeJSON(w, r, &request, 32<<10); err != nil || strings.TrimSpace(request.TransferToken) == "" {
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/invalid-transfer-request",
			Title: "转存请求格式无效", Status: http.StatusBadRequest,
			Code: "invalid_transfer_request",
		})
		return
	}
	principal := principalFromContext(r.Context())
	job, created, err := h.dependencies.Workflow.Enqueue(
		r.Context(), principal.UserID, request.TransferToken, strings.TrimSpace(r.Header.Get("Idempotency-Key")),
	)
	if err != nil {
		writeTransferProblem(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusAccepted
	}
	writeJSON(w, status, job)
}

func (h *handler) listTransfers(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Workflow == nil {
		writeTransferProblem(w, workflow.ErrUnavailable)
		return
	}
	limit := 50
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 1 || parsed > 100 {
			writeProblem(w, problem{
				Type:  "https://media-hub.local/problems/invalid-limit",
				Title: "列表数量无效", Status: http.StatusBadRequest,
				Code: "invalid_limit",
			})
			return
		}
		limit = parsed
	}
	archived := false
	if rawArchived := strings.TrimSpace(r.URL.Query().Get("archived")); rawArchived != "" {
		parsed, err := strconv.ParseBool(rawArchived)
		if err != nil {
			writeProblem(w, problem{
				Type: "https://media-hub.local/problems/invalid-archive-filter", Title: "归档筛选无效",
				Status: http.StatusBadRequest, Code: "invalid_archive_filter",
			})
			return
		}
		archived = parsed
	}
	principal := principalFromContext(r.Context())
	jobs, err := h.dependencies.Workflow.List(r.Context(), principal.UserID, limit, archived)
	if err != nil {
		writeTransferProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, transferListResponse{Transfers: jobs})
}

func (h *handler) getTransfer(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Workflow == nil {
		writeTransferProblem(w, workflow.ErrUnavailable)
		return
	}
	principal := principalFromContext(r.Context())
	job, err := h.dependencies.Workflow.Get(r.Context(), principal.UserID, r.PathValue("id"))
	if err != nil {
		writeTransferProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *handler) retryTransfer(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Workflow == nil {
		writeTransferProblem(w, workflow.ErrUnavailable)
		return
	}
	principal := principalFromContext(r.Context())
	job, err := h.dependencies.Workflow.Retry(r.Context(), principal.UserID, r.PathValue("id"))
	if err != nil {
		writeTransferProblem(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (h *handler) setTransferArchived(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Workflow == nil {
		writeTransferProblem(w, workflow.ErrUnavailable)
		return
	}
	var request setTransferArchivedRequest
	if err := decodeJSON(w, r, &request, 4<<10); err != nil {
		return
	}
	if request.Archived == nil {
		writeProblem(w, problem{
			Type: "https://media-hub.local/problems/invalid-transfer-archive", Title: "任务归档请求无效",
			Status: http.StatusBadRequest, Code: "invalid_transfer_archive",
		})
		return
	}
	principal := principalFromContext(r.Context())
	job, err := h.dependencies.Workflow.SetArchived(r.Context(), principal.UserID, r.PathValue("id"), *request.Archived)
	if err != nil {
		writeTransferProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func writeTransferProblem(w http.ResponseWriter, err error) {
	value := problem{
		Type:  "https://media-hub.local/problems/internal",
		Title: "转存任务处理失败", Status: http.StatusInternalServerError,
		Code: "internal_error",
	}
	switch {
	case errors.Is(err, workflow.ErrInvalidSelection):
		value.Type = "https://media-hub.local/problems/invalid-selection"
		value.Title = "资源选择已过期"
		value.Status = http.StatusBadRequest
		value.Code = "invalid_selection"
	case errors.Is(err, workflow.ErrInvalidIdempotency):
		value.Type = "https://media-hub.local/problems/invalid-idempotency-key"
		value.Title = "幂等键无效"
		value.Status = http.StatusBadRequest
		value.Code = "invalid_idempotency_key"
	case errors.Is(err, workflow.ErrUnavailable),
		errors.Is(err, workflow.ErrSourceUnavailable),
		errors.Is(err, workflow.ErrTargetUnavailable):
		value.Type = "https://media-hub.local/problems/workflow-unavailable"
		value.Title = "转存工作流尚未配置完整"
		value.Status = http.StatusConflict
		value.Code = "workflow_unavailable"
	case errors.Is(err, store.ErrIdempotencyConflict):
		value.Type = "https://media-hub.local/problems/idempotency-conflict"
		value.Title = "幂等键已用于其他请求"
		value.Status = http.StatusConflict
		value.Code = "idempotency_conflict"
	case errors.Is(err, store.ErrTransferNotFound):
		value.Type = "https://media-hub.local/problems/transfer-not-found"
		value.Title = "转存任务不存在"
		value.Status = http.StatusNotFound
		value.Code = "transfer_not_found"
	case errors.Is(err, store.ErrTransferNotRetryable):
		value.Type = "https://media-hub.local/problems/transfer-not-retryable"
		value.Title = "当前任务不能重试"
		value.Status = http.StatusConflict
		value.Code = "transfer_not_retryable"
	case errors.Is(err, store.ErrTransferNotArchivable):
		value.Type = "https://media-hub.local/problems/transfer-not-archivable"
		value.Title = "当前任务不能归档"
		value.Status = http.StatusConflict
		value.Code = "transfer_not_archivable"
	}
	writeProblem(w, value)
}
