package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"media-hub/backend/internal/strm"
)

func (h *handler) getSTRMStatus(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.STRM == nil {
		writeIntegrationUnavailable(w)
		return
	}
	status, err := h.dependencies.STRM.Status(r.Context())
	if err != nil {
		writeSTRMProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *handler) syncSTRMLibrary(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.STRM == nil {
		writeIntegrationUnavailable(w)
		return
	}
	var input struct {
		MediaType string `json:"mediaType"`
		Full      bool   `json:"full"`
		DryRun    bool   `json:"dryRun"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&input); err != nil && !errors.Is(err, io.EOF) {
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/invalid-request",
			Title: "STRM 同步请求无效", Status: http.StatusBadRequest,
			Code: "invalid_request",
		})
		return
	}
	mediaType := strings.ToLower(strings.TrimSpace(input.MediaType))
	if mediaType == "" {
		mediaType = "all"
	}
	if mediaType != "movie" && mediaType != "series" && mediaType != "all" {
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/invalid-media-type",
			Title: "媒体类型无效", Status: http.StatusBadRequest,
			Code: "invalid_media_type",
		})
		return
	}
	err := h.dependencies.STRM.EnqueueLibrarySync(strm.LibrarySyncInput{
		MediaType: mediaType, Full: input.Full, DryRun: input.DryRun,
	})
	if err != nil {
		writeSTRMProblem(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (h *handler) redirectSTRM(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.STRM == nil {
		writeIntegrationUnavailable(w)
		return
	}
	pickCode := strings.TrimSpace(r.URL.Query().Get("pickcode"))
	if pickCode == "" {
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/invalid-request",
			Title: "缺少播放凭证", Status: http.StatusBadRequest,
			Code: "invalid_request",
		})
		return
	}
	location, err := h.dependencies.STRM.Redirect(r.Context(), pickCode, r.PathValue("name"), r.UserAgent())
	if err != nil {
		writeSTRMProblem(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	http.Redirect(w, r, location, http.StatusFound)
}

func writeSTRMProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, strm.ErrInvalidRequest):
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/invalid-request",
			Title: "STRM 请求无效", Status: http.StatusBadRequest,
			Code: "invalid_request",
		})
	case errors.Is(err, strm.ErrBusy):
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/strm-busy",
			Title: "已有 STRM 同步在执行", Status: http.StatusConflict,
			Code: "strm_busy",
		})
	case errors.Is(err, strm.ErrAuthExpired):
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/integration-unauthorized",
			Title: "115 会话不可用", Status: http.StatusBadGateway,
			Code: "integration_unauthorized",
		})
	default:
		writeIntegrationProblem(w, err)
	}
}
