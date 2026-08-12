package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"media-hub/backend/internal/store"
	"media-hub/backend/internal/workflow"
)

type notificationListResponse struct {
	Notifications []workflow.Notification `json:"notifications"`
}

type retryNotificationRequest struct {
	Confirm string `json:"confirm"`
}

func (h *handler) listNotifications(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Workflow == nil {
		writeNotificationProblem(w, workflow.ErrUnavailable)
		return
	}
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writeProblem(w, problem{Title: "limit 必须是 1 到 100 的整数", Status: http.StatusBadRequest, Code: "invalid_limit"})
			return
		}
		limit = parsed
	}
	principal := principalFromContext(r.Context())
	items, err := h.dependencies.Workflow.ListNotifications(r.Context(), principal.UserID, limit)
	if err != nil {
		writeNotificationProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, notificationListResponse{Notifications: items})
}

func (h *handler) retryNotification(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Workflow == nil {
		writeNotificationProblem(w, workflow.ErrUnavailable)
		return
	}
	var request retryNotificationRequest
	if decodeJSON(w, r, &request, 4<<10) != nil {
		writeProblem(w, problem{Title: "通知重发确认无效", Status: http.StatusBadRequest, Code: "invalid_confirmation"})
		return
	}
	principal := principalFromContext(r.Context())
	item, err := h.dependencies.Workflow.RetryNotification(
		r.Context(), principal.UserID, r.PathValue("jobId"), r.PathValue("eventType"), request.Confirm,
	)
	if err != nil {
		writeNotificationProblem(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, item)
}

func writeNotificationProblem(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "notification_failed"
	message := "无法处理通知"
	switch {
	case errors.Is(err, workflow.ErrUnavailable):
		status = http.StatusServiceUnavailable
		code = "notification_unavailable"
		message = "通知服务未配置"
	case errors.Is(err, store.ErrNotificationNotFound):
		status = http.StatusNotFound
		code = "notification_not_found"
		message = "通知不存在"
	case errors.Is(err, store.ErrNotificationNotRetryable):
		status = http.StatusConflict
		code = "notification_not_retryable"
		message = "通知不可重发或确认值不匹配"
	}
	writeProblem(w, problem{Title: message, Status: status, Code: code})
}
