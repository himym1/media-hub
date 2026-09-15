package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"media-hub/backend/internal/playback"
)

type createDrive115PlaybackRequest struct {
	ParentID          string `json:"parentId"`
	FileID            string `json:"fileId"`
	PlaybackUserAgent string `json:"playbackUserAgent"`
}

type createEmbyPlaybackRequest struct {
	ItemID            string `json:"itemId"`
	PlaybackUserAgent string `json:"playbackUserAgent"`
}

type playbackSessionEventRequest struct {
	Event      playback.SessionEventType `json:"event"`
	PositionMS int64                     `json:"positionMs"`
	Paused     bool                      `json:"paused"`
}

func (h *handler) createDrive115Playback(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Playback == nil {
		writePlaybackProblem(w, playback.ErrUnavailable)
		return
	}
	var input createDrive115PlaybackRequest
	if !decodePlaybackRequest(w, r, &input) {
		return
	}
	value, err := h.dependencies.Playback.CreateDrive115(r.Context(), playback.Drive115Target{
		ParentID: input.ParentID,
		FileID:   input.FileID,
	}, input.PlaybackUserAgent)
	writePlaybackResult(w, value, err)
}

func (h *handler) createEmbyPlayback(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Playback == nil {
		writePlaybackProblem(w, playback.ErrUnavailable)
		return
	}
	var input createEmbyPlaybackRequest
	if !decodePlaybackRequest(w, r, &input) {
		return
	}
	principal := principalFromContext(r.Context())
	value, err := h.dependencies.Playback.CreateEmbyItem(
		r.Context(), principal.UserID, playback.EmbyItemTarget{ItemID: input.ItemID}, input.PlaybackUserAgent,
	)
	writePlaybackResult(w, value, err)
}

func (h *handler) redirectEmbyStream(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Playback == nil {
		writePlaybackProblem(w, playback.ErrUnavailable)
		return
	}
	location, err := h.dependencies.Playback.RedirectEmby(r.Context(), r.URL.Query().Get("ticket"))
	if err != nil {
		writePlaybackProblem(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	http.Redirect(w, r, location, http.StatusFound)
}

func (h *handler) reportPlaybackSession(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Playback == nil {
		writePlaybackProblem(w, playback.ErrUnavailable)
		return
	}
	var input playbackSessionEventRequest
	if !decodePlaybackRequest(w, r, &input) {
		return
	}
	err := h.dependencies.Playback.Report(
		r.Context(),
		principalFromContext(r.Context()).UserID,
		r.PathValue("id"),
		playback.SessionEvent{Type: input.Event, PositionMS: input.PositionMS, Paused: input.Paused},
	)
	if err != nil {
		writePlaybackProblem(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodePlaybackRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	if r.Body == nil || json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(target) != nil {
		writePlaybackProblem(w, playback.ErrInvalidRequest)
		return false
	}
	return true
}

func writePlaybackResult(w http.ResponseWriter, value playback.Descriptor, err error) {
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
		writeProblem(w, problem{Title: "媒体不存在或不可播放", Status: http.StatusNotFound, Code: "playable_media_not_found"})
	case errors.Is(err, playback.ErrSourceNotConfigured):
		writeProblem(w, problem{Title: "播放源尚未配置", Status: http.StatusServiceUnavailable, Code: "playback_source_not_configured"})
	case errors.Is(err, playback.ErrSourceUnauthorized):
		writeProblem(w, problem{Title: "播放源授权已失效", Status: http.StatusBadGateway, Code: "playback_source_unauthorized"})
	default:
		writeProblem(w, problem{Title: "暂时无法直接播放", Status: http.StatusBadGateway, Code: "direct_playback_unavailable"})
	}
}
