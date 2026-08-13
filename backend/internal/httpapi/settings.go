package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"media-hub/backend/internal/settings"
)

func (h *handler) getProviderSettings(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Settings == nil {
		writeProviderSettingsProblem(w, settings.ErrUnavailable)
		return
	}
	value, err := h.dependencies.Settings.Get(r.Context(), principalFromContext(r.Context()).UserID)
	if err != nil {
		writeProviderSettingsProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (h *handler) updateProviderSettings(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Settings == nil {
		writeProviderSettingsProblem(w, settings.ErrUnavailable)
		return
	}
	var input settings.Update
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeProviderSettingsProblem(w, settings.ErrInvalid)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeProviderSettingsProblem(w, settings.ErrInvalid)
		return
	}
	value, err := h.dependencies.Settings.Update(r.Context(), principalFromContext(r.Context()).UserID, input)
	if err != nil {
		writeProviderSettingsProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func writeProviderSettingsProblem(w http.ResponseWriter, err error) {
	if errors.Is(err, settings.ErrInvalid) {
		writeProblem(w, problem{Title: "服务设置无效", Status: http.StatusUnprocessableEntity, Code: "invalid_provider_settings"})
		return
	}
	if errors.Is(err, settings.ErrActiveProviderOperations) {
		writeProblem(w, problem{Title: "存在未完成或可恢复的 Provider 操作，请处理后再修改服务设置", Status: http.StatusConflict, Code: "active_provider_operations"})
		return
	}
	if errors.Is(err, settings.ErrUnavailable) {
		writeProblem(w, problem{Title: "服务设置不可用", Status: http.StatusServiceUnavailable, Code: "provider_settings_unavailable"})
		return
	}
	writeProblem(w, problem{Title: "保存服务设置失败", Status: http.StatusInternalServerError, Code: "provider_settings_failed"})
}
