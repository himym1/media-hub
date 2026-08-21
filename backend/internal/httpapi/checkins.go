package httpapi

import (
	"context"
	"errors"
	"net/http"

	"media-hub/backend/internal/checkin"
	"media-hub/backend/internal/store"
)

type SourceCheckInService interface {
	Status(context.Context) (checkin.Status, error)
	Retry(context.Context, string) (checkin.Item, error)
}

func (h *handler) listSourceCheckIns(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.SourceCheckIns == nil {
		writeIntegrationUnavailable(w)
		return
	}
	status, err := h.dependencies.SourceCheckIns.Status(r.Context())
	if err != nil {
		writeCheckInProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *handler) retrySourceCheckIn(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.SourceCheckIns == nil {
		writeIntegrationUnavailable(w)
		return
	}
	item, err := h.dependencies.SourceCheckIns.Retry(r.Context(), r.PathValue("id"))
	if err != nil {
		writeCheckInProblem(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, item)
}

func writeCheckInProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, checkin.ErrInvalidSource):
		writeProblem(w, problem{
			Type: "https://media-hub.local/problems/invalid-source-id", Title: "资源源标识无效",
			Status: http.StatusBadRequest, Code: "invalid_source_id",
		})
	case errors.Is(err, store.ErrCheckInNotFound):
		writeProblem(w, problem{
			Type: "https://media-hub.local/problems/source-checkin-not-found", Title: "该资源源不支持签到",
			Status: http.StatusNotFound, Code: "source_checkin_not_found",
		})
	case errors.Is(err, store.ErrCheckInNotRetryable):
		writeProblem(w, problem{
			Type: "https://media-hub.local/problems/source-checkin-not-retryable", Title: "当前签到正在执行",
			Status: http.StatusConflict, Code: "source_checkin_not_retryable",
		})
	case errors.Is(err, checkin.ErrUnavailable):
		writeIntegrationUnavailable(w)
	default:
		writeProblem(w, problem{
			Type: "https://media-hub.local/problems/source-checkin-failed", Title: "资源源签到失败",
			Status: http.StatusInternalServerError, Code: "source_checkin_failed",
		})
	}
}
