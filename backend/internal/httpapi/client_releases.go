package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"media-hub/backend/internal/androidrelease"
)

func (h *handler) getLatestAndroidRelease(w http.ResponseWriter, r *http.Request) {
	release, err := h.dependencies.AndroidReleases.Latest(r.Context())
	if err != nil {
		writeAndroidReleaseProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, release)
}

func (h *handler) downloadAndroidRelease(w http.ResponseWriter, r *http.Request) {
	versionCode, err := strconv.Atoi(r.PathValue("versionCode"))
	if err != nil || versionCode <= 0 {
		writeProblem(w, problem{Title: "Android 版本编号无效", Status: http.StatusBadRequest, Code: "invalid_version_code"})
		return
	}
	release, file, err := h.dependencies.AndroidReleases.OpenAPK(r.Context(), versionCode)
	if err != nil {
		writeAndroidReleaseProblem(w, err)
		return
	}
	defer file.Close()

	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Minute))
	w.Header().Set("Content-Type", "application/vnd.android.package-archive")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="media-hub-%d.apk"`, release.VersionCode))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Checksum-SHA256", release.SHA256)
	http.ServeContent(w, r, fmt.Sprintf("media-hub-%d.apk", release.VersionCode), release.PublishedAt, file)
}

func writeAndroidReleaseProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, androidrelease.ErrNotConfigured), errors.Is(err, androidrelease.ErrNotFound):
		writeProblem(w, problem{Title: "当前没有可用的 Android 更新", Status: http.StatusNotFound, Code: "android_release_unavailable"})
	case errors.Is(err, androidrelease.ErrInvalid):
		writeProblem(w, problem{Title: "Android 更新包校验失败", Status: http.StatusServiceUnavailable, Code: "android_release_invalid"})
	default:
		writeProblem(w, problem{Title: "无法读取 Android 更新包", Status: http.StatusInternalServerError, Code: "android_release_failed"})
	}
}
