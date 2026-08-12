package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"media-hub/backend/internal/store"
	"media-hub/backend/internal/subscription"
)

type subscriptionListResponse struct {
	Subscriptions []subscription.Subscription `json:"subscriptions"`
}

type subscriptionRunListResponse struct {
	Runs []subscription.Run `json:"runs"`
}

type setSubscriptionEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *handler) createSubscription(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Subscriptions == nil {
		writeSubscriptionProblem(w, subscription.ErrUnavailable)
		return
	}
	var request subscription.CreateInput
	if err := decodeJSON(w, r, &request, 64<<10); err != nil {
		writeSubscriptionProblem(w, subscription.ErrInvalidSubscription)
		return
	}
	principal := principalFromContext(r.Context())
	item, err := h.dependencies.Subscriptions.Create(r.Context(), principal.UserID, request)
	if err != nil {
		writeSubscriptionProblem(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *handler) listSubscriptions(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Subscriptions == nil {
		writeSubscriptionProblem(w, subscription.ErrUnavailable)
		return
	}
	principal := principalFromContext(r.Context())
	items, err := h.dependencies.Subscriptions.List(r.Context(), principal.UserID)
	if err != nil {
		writeSubscriptionProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, subscriptionListResponse{Subscriptions: items})
}

func (h *handler) exportSubscriptions(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Subscriptions == nil {
		writeSubscriptionProblem(w, subscription.ErrUnavailable)
		return
	}
	principal := principalFromContext(r.Context())
	backup, err := h.dependencies.Subscriptions.Export(r.Context(), principal.UserID)
	if err != nil {
		writeSubscriptionProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, backup)
}

func (h *handler) importSubscriptions(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Subscriptions == nil {
		writeSubscriptionProblem(w, subscription.ErrUnavailable)
		return
	}
	var backup subscription.Backup
	if err := decodeJSON(w, r, &backup, 2<<20); err != nil {
		writeSubscriptionProblem(w, subscription.ErrInvalidSubscription)
		return
	}
	principal := principalFromContext(r.Context())
	result, err := h.dependencies.Subscriptions.Import(r.Context(), principal.UserID, backup)
	if err != nil {
		writeSubscriptionProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) setSubscriptionBatchEnabled(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Subscriptions == nil {
		writeSubscriptionProblem(w, subscription.ErrUnavailable)
		return
	}
	var request subscription.BatchEnabledInput
	if err := decodeJSON(w, r, &request, 128<<10); err != nil {
		writeSubscriptionProblem(w, subscription.ErrInvalidSubscription)
		return
	}
	principal := principalFromContext(r.Context())
	items, err := h.dependencies.Subscriptions.SetBatchEnabled(r.Context(), principal.UserID, request)
	if err != nil {
		writeSubscriptionProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, subscriptionListResponse{Subscriptions: items})
}

func (h *handler) getSubscription(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Subscriptions == nil {
		writeSubscriptionProblem(w, subscription.ErrUnavailable)
		return
	}
	principal := principalFromContext(r.Context())
	item, err := h.dependencies.Subscriptions.Get(r.Context(), principal.UserID, r.PathValue("id"))
	if err != nil {
		writeSubscriptionProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *handler) updateSubscription(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Subscriptions == nil {
		writeSubscriptionProblem(w, subscription.ErrUnavailable)
		return
	}
	var request subscription.UpdateInput
	if err := decodeJSON(w, r, &request, 64<<10); err != nil {
		writeSubscriptionProblem(w, subscription.ErrInvalidSubscription)
		return
	}
	principal := principalFromContext(r.Context())
	item, err := h.dependencies.Subscriptions.Update(r.Context(), principal.UserID, r.PathValue("id"), request)
	if err != nil {
		writeSubscriptionProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *handler) deleteSubscription(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Subscriptions == nil {
		writeSubscriptionProblem(w, subscription.ErrUnavailable)
		return
	}
	principal := principalFromContext(r.Context())
	if err := h.dependencies.Subscriptions.Delete(r.Context(), principal.UserID, r.PathValue("id")); err != nil {
		writeSubscriptionProblem(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) setSubscriptionEnabled(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Subscriptions == nil {
		writeSubscriptionProblem(w, subscription.ErrUnavailable)
		return
	}
	var request setSubscriptionEnabledRequest
	if err := decodeJSON(w, r, &request, 4<<10); err != nil {
		writeSubscriptionProblem(w, subscription.ErrInvalidSubscription)
		return
	}
	principal := principalFromContext(r.Context())
	item, err := h.dependencies.Subscriptions.SetEnabled(r.Context(), principal.UserID, r.PathValue("id"), request.Enabled)
	if err != nil {
		writeSubscriptionProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *handler) runSubscription(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Subscriptions == nil {
		writeSubscriptionProblem(w, subscription.ErrUnavailable)
		return
	}
	principal := principalFromContext(r.Context())
	run, err := h.dependencies.Subscriptions.RunNow(r.Context(), principal.UserID, r.PathValue("id"))
	if err != nil {
		writeSubscriptionProblem(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, run)
}

func (h *handler) listSubscriptionRuns(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Subscriptions == nil {
		writeSubscriptionProblem(w, subscription.ErrUnavailable)
		return
	}
	limit := 50
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 1 || parsed > 100 {
			writeProblem(w, problem{
				Type: "https://media-hub.local/problems/invalid-limit", Title: "列表数量无效",
				Status: http.StatusBadRequest, Code: "invalid_limit",
			})
			return
		}
		limit = parsed
	}
	principal := principalFromContext(r.Context())
	runs, err := h.dependencies.Subscriptions.Runs(r.Context(), principal.UserID, r.PathValue("id"), limit)
	if err != nil {
		writeSubscriptionProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, subscriptionRunListResponse{Runs: runs})
}

func writeSubscriptionProblem(w http.ResponseWriter, err error) {
	value := problem{
		Type: "https://media-hub.local/problems/internal", Title: "订阅操作失败",
		Status: http.StatusInternalServerError, Code: "internal_error",
	}
	switch {
	case errors.Is(err, subscription.ErrInvalidSubscription):
		value.Type = "https://media-hub.local/problems/invalid-subscription"
		value.Title = "订阅配置无效"
		value.Status = http.StatusBadRequest
		value.Code = "invalid_subscription"
	case errors.Is(err, subscription.ErrUnavailable):
		value.Type = "https://media-hub.local/problems/subscriptions-unavailable"
		value.Title = "订阅服务不可用"
		value.Status = http.StatusServiceUnavailable
		value.Code = "subscriptions_unavailable"
	case errors.Is(err, store.ErrSubscriptionNotFound):
		value.Type = "https://media-hub.local/problems/subscription-not-found"
		value.Title = "订阅不存在"
		value.Status = http.StatusNotFound
		value.Code = "subscription_not_found"
	case errors.Is(err, store.ErrSubscriptionConflict):
		value.Type = "https://media-hub.local/problems/subscription-conflict"
		value.Title = "相同媒体和季号已经订阅"
		value.Status = http.StatusConflict
		value.Code = "subscription_conflict"
	case errors.Is(err, store.ErrSubscriptionRunActive):
		value.Type = "https://media-hub.local/problems/subscription-run-active"
		value.Title = "订阅已有执行中的任务"
		value.Status = http.StatusConflict
		value.Code = "subscription_run_active"
	}
	writeProblem(w, value)
}
