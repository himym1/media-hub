package httpapi

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"media-hub/backend/internal/drive115"
	"media-hub/backend/internal/store"
)

type drive115AuthorizationResponse struct {
	ID        string    `json:"id"`
	QRImage   string    `json:"qrImage,omitempty"`
	State     string    `json:"state"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (h *handler) startDrive115Authorization(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Drive115Auth == nil {
		h.writeDrive115AuthorizationError(w, drive115.ErrAuthorizationUnavailable)
		return
	}
	principal := principalFromContext(r.Context())
	value, err := h.dependencies.Drive115Auth.Start(r.Context(), principal.UserID)
	if err != nil {
		h.writeDrive115AuthorizationError(w, err)
		return
	}
	png, err := qrcode.Encode(value.QRCode, qrcode.Medium, 256)
	if err != nil {
		h.writeDrive115AuthorizationError(w, drive115.ErrUpstreamResponse)
		return
	}
	writeJSON(w, http.StatusCreated, drive115AuthorizationResponse{
		ID: value.ID, QRImage: "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
		State: value.State, ExpiresAt: value.ExpiresAt,
	})
}

func (h *handler) pollDrive115Authorization(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Drive115Auth == nil {
		h.writeDrive115AuthorizationError(w, drive115.ErrAuthorizationUnavailable)
		return
	}
	principal := principalFromContext(r.Context())
	value, err := h.dependencies.Drive115Auth.Poll(r.Context(), principal.UserID, r.PathValue("id"))
	if err != nil {
		h.writeDrive115AuthorizationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, drive115AuthorizationResponse{
		ID: value.ID, State: value.State, ExpiresAt: value.ExpiresAt,
	})
}

func (h *handler) writeDrive115AuthorizationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrProviderChallengeNotFound):
		writeProblem(w, problem{Type: "https://media-hub.local/problems/not-found", Title: "115 授权请求不存在", Status: http.StatusNotFound, Code: "authorization_not_found"})
	case errors.Is(err, drive115.ErrAuthorizationUnavailable):
		writeProblem(w, problem{Type: "https://media-hub.local/problems/authorization-unavailable", Title: "115 扫码授权尚未配置", Status: http.StatusServiceUnavailable, Code: "authorization_unavailable"})
	case errors.Is(err, drive115.ErrAuthorizationExpired):
		writeProblem(w, problem{Type: "https://media-hub.local/problems/authorization-expired", Title: "115 授权请求已过期", Status: http.StatusGone, Code: "authorization_expired"})
	case errors.Is(err, drive115.ErrNotConfigured):
		writeProblem(w, problem{Title: "115 尚未授权", Status: http.StatusServiceUnavailable, Code: "not_configured"})
	case errors.Is(err, drive115.ErrUnauthorized):
		writeProblem(w, problem{Title: "115 授权已失效", Status: http.StatusBadGateway, Code: "provider_unauthorized"})
	default:
		writeProblem(w, problem{Type: "https://media-hub.local/problems/provider-unavailable", Title: "115 授权服务暂时不可用", Status: http.StatusBadGateway, Code: "provider_unavailable"})
	}
}

func (h *handler) listDrive115Files(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Drive115 == nil {
		writeProblem(w, problem{Title: "115 未配置", Status: http.StatusServiceUnavailable, Code: "not_configured"})
		return
	}
	parentID := strings.TrimSpace(r.URL.Query().Get("parentId"))
	if parentID == "" {
		parentID = "0"
	}
	if _, err := strconv.ParseUint(parentID, 10, 64); err != nil {
		writeProblem(w, problem{Title: "parentId 必须是 115 目录 ID", Status: http.StatusBadRequest, Code: "invalid_parent_id"})
		return
	}
	limit, offset := 50, 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 200 {
			writeProblem(w, problem{Title: "limit 必须是 1 到 200 的整数", Status: http.StatusBadRequest, Code: "invalid_limit"})
			return
		}
		limit = value
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			writeProblem(w, problem{Title: "offset 必须是非负整数", Status: http.StatusBadRequest, Code: "invalid_offset"})
			return
		}
		offset = value
	}
	items, total, err := h.dependencies.Drive115.ListFiles(r.Context(), parentID, limit, offset)
	if err != nil {
		h.writeDrive115AuthorizationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}
