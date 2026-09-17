package httpapi

import (
	"errors"
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
	parsed, err := adapter.ParseAdultImport(request.URL, request.ReceiveCode)
	if err != nil || parsed.Kind == "" {
		writeProblem(w, adultImportProblem(err))
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
		r.Context(), principal.UserID, title, request.URL, request.ReceiveCode, strings.TrimSpace(r.Header.Get("Idempotency-Key")),
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

func adultImportProblem(err error) problem {
	detail := "只接受 115 分享、磁力、电驴或可下载的视频地址"
	switch {
	case errors.Is(err, adapter.ErrOtherCloudImport):
		detail = "夸克、阿里云、百度网盘分享不能直接导入。请贴 115 分享、磁力或视频直链"
	case errors.Is(err, adapter.ErrNeed115ShareOrFile):
		detail = "这不是 115 分享链接。请贴 https://115.com/s/…，或视频文件地址 / 磁力"
	}
	return problem{
		Type:  "https://media-hub.local/problems/invalid-share-import",
		Title: "导入链接无效", Status: http.StatusBadRequest,
		Code: "invalid_share_import", Detail: detail,
	}
}
