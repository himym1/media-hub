package httpapi

import (
	"net/http"
	"strings"

	"media-hub/backend/internal/adapter"
	"media-hub/backend/internal/workflow"
)

type createShareImportRequest struct {
	URL         string `json:"url"`
	ReceiveCode string `json:"receiveCode"`
	Title       string `json:"title"`
}

func (h *handler) createShareImport(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Workflow == nil {
		writeTransferProblem(w, workflow.ErrUnavailable)
		return
	}
	var request createShareImportRequest
	if err := decodeJSON(w, r, &request, 32<<10); err != nil {
		return
	}
	shareCode, receiveCode, err := adapter.Parse115ShareURL(request.URL, request.ReceiveCode)
	if err != nil || strings.TrimSpace(shareCode) == "" {
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/invalid-share-import",
			Title: "115 分享链接无效", Status: http.StatusBadRequest,
			Code: "invalid_share_import", Detail: "只接受 115.com / 115cdn.com / anxia.com 的分享链接",
		})
		return
	}
	title := strings.TrimSpace(request.Title)
	if len([]rune(title)) > 300 {
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/invalid-share-import",
			Title: "标题过长", Status: http.StatusBadRequest,
			Code: "invalid_share_import",
		})
		return
	}
	principal := principalFromContext(r.Context())
	job, created, err := h.dependencies.Workflow.EnqueueShareImport(
		r.Context(), principal.UserID, title, shareCode, receiveCode, strings.TrimSpace(r.Header.Get("Idempotency-Key")),
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
