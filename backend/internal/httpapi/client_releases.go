package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"media-hub/backend/internal/androidrelease"
	"media-hub/backend/internal/desktoprelease"
)

func (h *handler) getLatestAndroidRelease(w http.ResponseWriter, r *http.Request) {
	release, err := h.dependencies.AndroidReleases.Latest(r.Context())
	if err != nil {
		writeAndroidReleaseProblem(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
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

func (h *handler) getLatestDesktopRelease(w http.ResponseWriter, r *http.Request) {
	platform, err := desktoprelease.ParsePlatform(r.URL.Query().Get("platform"))
	if err != nil {
		writeProblem(w, problem{Title: "桌面端平台无效", Status: http.StatusBadRequest, Code: "invalid_desktop_platform"})
		return
	}
	release, err := h.dependencies.DesktopReleases.Latest(r.Context(), platform)
	if err != nil {
		writeDesktopReleaseProblem(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, release)
}

func (h *handler) downloadDesktopRelease(w http.ResponseWriter, r *http.Request) {
	versionCode, err := strconv.Atoi(r.PathValue("versionCode"))
	if err != nil || versionCode <= 0 {
		writeProblem(w, problem{Title: "桌面端版本编号无效", Status: http.StatusBadRequest, Code: "invalid_version_code"})
		return
	}
	platform := desktoprelease.PlatformWindows
	if strings.HasSuffix(r.URL.Path, "/dmg") {
		platform = desktoprelease.PlatformDarwin
	}
	release, file, err := h.dependencies.DesktopReleases.OpenInstaller(r.Context(), versionCode, platform)
	if err != nil {
		writeDesktopReleaseProblem(w, err)
		return
	}
	defer file.Close()

	filename := fmt.Sprintf("media-hub-%d.exe", release.VersionCode)
	contentType := "application/octet-stream"
	if platform == desktoprelease.PlatformDarwin {
		filename = fmt.Sprintf("media-hub-%d.dmg", release.VersionCode)
		contentType = "application/x-apple-diskimage"
	}
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(10 * time.Minute))
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Checksum-SHA256", release.SHA256)
	http.ServeContent(w, r, filename, release.PublishedAt, file)
}

func writeDesktopReleaseProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, desktoprelease.ErrNotConfigured), errors.Is(err, desktoprelease.ErrNotFound):
		writeProblem(w, problem{Title: "当前没有可用的桌面更新", Status: http.StatusNotFound, Code: "desktop_release_unavailable"})
	case errors.Is(err, desktoprelease.ErrInvalid):
		writeProblem(w, problem{Title: "桌面更新包校验失败", Status: http.StatusServiceUnavailable, Code: "desktop_release_invalid"})
	default:
		writeProblem(w, problem{Title: "无法读取桌面更新包", Status: http.StatusInternalServerError, Code: "desktop_release_failed"})
	}
}
