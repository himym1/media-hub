package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"media-hub/backend/internal/playback"
)

type createPlaybackRequest struct {
	ParentID string `json:"parentId"`
	FileID   string `json:"fileId"`
}

func (h *handler) createPlayback(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Playback == nil {
		writePlaybackProblem(w, playback.ErrUnavailable)
		return
	}
	var input createPlaybackRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&input); err != nil {
		writePlaybackProblem(w, playback.ErrInvalidRequest)
		return
	}
	value, err := h.dependencies.Playback.Create(r.Context(), input.ParentID, input.FileID)
	if err != nil {
		writePlaybackProblem(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

func writePlaybackProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, playback.ErrInvalidRequest):
		writeProblem(w, problem{Title: "播放请求无效", Status: http.StatusBadRequest, Code: "invalid_playback_request"})
	case errors.Is(err, playback.ErrNotFound):
		writeProblem(w, problem{Title: "视频文件不存在或不可播放", Status: http.StatusNotFound, Code: "playable_media_not_found"})
	case errors.Is(err, playback.ErrSourceNotConfigured):
		writeProblem(w, problem{Title: "115 尚未授权", Status: http.StatusServiceUnavailable, Code: "not_configured"})
	case errors.Is(err, playback.ErrSourceUnauthorized):
		writeProblem(w, problem{Title: "115 授权已失效", Status: http.StatusBadGateway, Code: "provider_unauthorized"})
	default:
		writeProblem(w, problem{Title: "暂时无法创建播放会话", Status: http.StatusBadGateway, Code: "playback_unavailable"})
	}
}
