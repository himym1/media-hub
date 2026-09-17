package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"media-hub/backend/internal/captureupload"
	"media-hub/backend/internal/drive115"
	"media-hub/backend/internal/workflow"
)

type initCaptureUploadRequest struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	Title    string `json:"title"`
}

type completeCaptureUploadRequest struct {
	DestinationID string `json:"destinationId"`
	Filename      string `json:"filename"`
	Title         string `json:"title"`
}

func (h *handler) initCaptureUpload(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.CaptureUploads == nil {
		writeCaptureUploadProblem(w, captureupload.ErrUnavailable)
		return
	}
	var request initCaptureUploadRequest
	if err := decodeJSON(w, r, &request, 32<<10); err != nil {
		writeCaptureUploadProblem(w, captureupload.ErrInvalid)
		return
	}
	ticket, err := h.dependencies.CaptureUploads.Init(r.Context(), request.Filename, request.Size, request.Title)
	if err != nil {
		writeCaptureUploadProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ticket)
}

func (h *handler) completeCaptureUpload(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.CaptureUploads == nil {
		writeCaptureUploadProblem(w, captureupload.ErrUnavailable)
		return
	}
	var request completeCaptureUploadRequest
	if err := decodeJSON(w, r, &request, 32<<10); err != nil {
		writeCaptureUploadProblem(w, captureupload.ErrInvalid)
		return
	}
	job, created, err := h.dependencies.CaptureUploads.Complete(
		r.Context(),
		principalFromContext(r.Context()).UserID,
		request.DestinationID,
		request.Filename,
		request.Title,
		strings.TrimSpace(r.Header.Get("Idempotency-Key")),
	)
	if err != nil {
		writeCaptureUploadProblem(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusAccepted
	}
	writeJSON(w, status, job)
}

func writeCaptureUploadProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, captureupload.ErrUnavailable):
		writeProblem(w, problem{Title: "网页抓取上传未配置", Status: http.StatusServiceUnavailable, Code: "not_configured"})
	case errors.Is(err, captureupload.ErrInvalid):
		writeProblem(w, problem{Title: "抓取文件无效", Status: http.StatusBadRequest, Code: "invalid_capture_upload"})
	case errors.Is(err, captureupload.ErrNotReady):
		writeProblem(w, problem{Title: "115 还没有收到抓取文件", Status: http.StatusConflict, Code: "capture_upload_not_ready"})
	case errors.Is(err, workflow.ErrTargetUnavailable), errors.Is(err, workflow.ErrUnavailable), errors.Is(err, workflow.ErrSourceUnavailable):
		writeTransferProblem(w, err)
	case errors.Is(err, workflow.ErrInvalidIdempotency), errors.Is(err, workflow.ErrInvalidSelection):
		writeTransferProblem(w, err)
	case errors.Is(err, drive115.ErrNotConfigured):
		writeProblem(w, problem{Title: "115 尚未授权", Status: http.StatusServiceUnavailable, Code: "not_configured"})
	case errors.Is(err, drive115.ErrUnauthorized):
		writeProblem(w, problem{Title: "115 授权已失效", Status: http.StatusBadGateway, Code: "provider_unauthorized"})
	default:
		writeProblem(w, problem{Title: "无法向 115 申请上传", Status: http.StatusBadGateway, Code: "provider_unavailable"})
	}
}
