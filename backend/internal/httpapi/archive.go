package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"media-hub/backend/internal/archive"
	"media-hub/backend/internal/store"
	"net/http"
	"strconv"
)

func (h *handler) previewArchive(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ParentID string `json:"parentId"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&input); err != nil {
		writeArchiveProblem(w, archive.ErrInvalidPlan)
		return
	}
	values, err := h.dependencies.Archive.Preview(r.Context(), input.ParentID)
	if err != nil {
		writeArchiveProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"suggestions": values})
}
func (h *handler) createArchivePlan(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Steps []archive.Step `json:"steps"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 256<<10)).Decode(&input); err != nil {
		writeArchiveProblem(w, archive.ErrInvalidPlan)
		return
	}
	value, err := h.dependencies.Archive.Create(r.Context(), principalFromContext(r.Context()).UserID, input.Steps)
	if err != nil {
		writeArchiveProblem(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}
func (h *handler) listArchivePlans(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			writeArchiveProblem(w, archive.ErrInvalidPlan)
			return
		}
		limit = value
	}
	values, err := h.dependencies.Archive.List(r.Context(), principalFromContext(r.Context()).UserID, limit)
	if err != nil {
		writeArchiveProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": values})
}
func (h *handler) getArchivePlan(w http.ResponseWriter, r *http.Request) {
	value, err := h.dependencies.Archive.Get(r.Context(), principalFromContext(r.Context()).UserID, r.PathValue("id"))
	if err != nil {
		writeArchiveProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (h *handler) confirmArchivePlan(w http.ResponseWriter, r *http.Request) {
	h.changeArchivePlan(w, r, false)
}
func (h *handler) retryArchivePlan(w http.ResponseWriter, r *http.Request) {
	h.changeArchivePlan(w, r, true)
}
func (h *handler) changeArchivePlan(w http.ResponseWriter, r *http.Request, retry bool) {
	var input struct {
		Confirmation string `json:"confirmation"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&input); err != nil {
		writeArchiveProblem(w, archive.ErrInvalidPlan)
		return
	}
	principal := principalFromContext(r.Context())
	var value archive.Plan
	var err error
	if retry {
		value, err = h.dependencies.Archive.Retry(r.Context(), principal.UserID, r.PathValue("id"), input.Confirmation)
	} else {
		value, err = h.dependencies.Archive.Confirm(r.Context(), principal.UserID, r.PathValue("id"), input.Confirmation)
	}
	if err != nil {
		writeArchiveProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func writeArchiveProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, archive.ErrUnavailable):
		writeProblem(w, problem{Title: "归档整理未配置", Status: http.StatusServiceUnavailable, Code: "not_configured"})
	case errors.Is(err, archive.ErrInvalidPlan):
		writeProblem(w, problem{Title: "归档计划无效", Status: http.StatusBadRequest, Code: "invalid_plan"})
	case errors.Is(err, store.ErrArchivePlanNotFound):
		writeProblem(w, problem{Title: "归档计划不存在或不可执行", Status: http.StatusNotFound, Code: "plan_not_found"})
	default:
		writeProblem(w, problem{Title: "归档服务暂时不可用", Status: http.StatusInternalServerError, Code: "archive_unavailable"})
	}
}
