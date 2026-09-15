package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"media-hub/backend/internal/androidrelease"
	"media-hub/backend/internal/desktoprelease"
)

type androidReleaseStub struct {
	release androidrelease.Release
	path    string
}

func (s androidReleaseStub) Latest(context.Context) (androidrelease.Release, error) {
	return s.release, nil
}

func (s androidReleaseStub) OpenAPK(context.Context, int) (androidrelease.Release, io.ReadSeekCloser, error) {
	file, err := os.Open(s.path)
	return s.release, file, err
}

func TestAndroidReleaseRoutesRequireAuthentication(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/client/android/releases/latest", nil)
	NewRouter("test", Dependencies{Auth: authStub{}, AndroidReleases: androidReleaseStub{}}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestLatestAndroidReleaseAndDownload(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "release.apk")
	if err := os.WriteFile(path, []byte("apk"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider := androidReleaseStub{
		path: path,
		release: androidrelease.Release{
			VersionCode: 6000, VersionName: "0.6.0", SHA256: "checksum", SizeBytes: 3,
			PublishedAt:  time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC),
			DownloadPath: "/api/v1/client/android/releases/6000/apk",
		},
	}
	router := NewRouter("test", Dependencies{Auth: authStub{}, AndroidReleases: provider})

	latest := httptest.NewRecorder()
	router.ServeHTTP(latest, authenticatedRequest("GET", "/api/v1/client/android/releases/latest"))
	if latest.Code != 200 {
		t.Fatalf("latest status = %d", latest.Code)
	}
	if latest.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("latest cache = %q", latest.Header().Get("Cache-Control"))
	}

	download := httptest.NewRecorder()
	router.ServeHTTP(download, authenticatedRequest("GET", "/api/v1/client/android/releases/6000/apk"))
	if download.Code != 200 || download.Body.String() != "apk" {
		t.Fatalf("download status = %d body = %q", download.Code, download.Body.String())
	}
	if download.Header().Get("X-Checksum-SHA256") != "checksum" {
		t.Fatal("missing checksum header")
	}
}

type desktopReleaseStub struct {
	release desktoprelease.Release
	path    string
}

func (s desktopReleaseStub) Latest(_ context.Context, _ desktoprelease.Platform) (desktoprelease.Release, error) {
	return s.release, nil
}

func (s desktopReleaseStub) OpenInstaller(_ context.Context, _ int, _ desktoprelease.Platform) (desktoprelease.Release, io.ReadSeekCloser, error) {
	file, err := os.Open(s.path)
	return s.release, file, err
}

func TestDesktopReleaseRoutesRequireAuthentication(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/client/desktop/releases/latest", nil)
	NewRouter("test", Dependencies{Auth: authStub{}, DesktopReleases: desktopReleaseStub{}}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestLatestDesktopReleaseAndDownload(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "release.exe")
	if err := os.WriteFile(path, []byte("exe"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider := desktopReleaseStub{
		path: path,
		release: desktoprelease.Release{
			VersionCode: 20038, VersionName: "0.20.38", SHA256: "desktop-checksum", SizeBytes: 3,
			PublishedAt:  time.Date(2026, 9, 14, 8, 0, 0, 0, time.UTC),
			DownloadPath: "/api/v1/client/desktop/releases/20038/installer",
		},
	}
	router := NewRouter("test", Dependencies{Auth: authStub{}, DesktopReleases: provider})

	latest := httptest.NewRecorder()
	router.ServeHTTP(latest, authenticatedRequest("GET", "/api/v1/client/desktop/releases/latest"))
	if latest.Code != 200 {
		t.Fatalf("latest status = %d", latest.Code)
	}
	if latest.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("latest cache = %q", latest.Header().Get("Cache-Control"))
	}

	download := httptest.NewRecorder()
	router.ServeHTTP(download, authenticatedRequest("GET", "/api/v1/client/desktop/releases/20038/installer"))
	if download.Code != 200 || download.Body.String() != "exe" {
		t.Fatalf("download status = %d body = %q", download.Code, download.Body.String())
	}
	if download.Header().Get("X-Checksum-SHA256") != "desktop-checksum" {
		t.Fatal("missing checksum header")
	}

	darwin := desktopReleaseStub{
		path: path,
		release: desktoprelease.Release{
			VersionCode: 20038, VersionName: "0.20.38", SHA256: "darwin-checksum", SizeBytes: 3,
			PublishedAt:  time.Date(2026, 9, 14, 8, 0, 0, 0, time.UTC),
			DownloadPath: "/api/v1/client/desktop/releases/20038/dmg",
		},
	}
	darwinRouter := NewRouter("test", Dependencies{Auth: authStub{}, DesktopReleases: darwin})
	latestDarwin := httptest.NewRecorder()
	darwinRouter.ServeHTTP(latestDarwin, authenticatedRequest("GET", "/api/v1/client/desktop/releases/latest?platform=darwin"))
	if latestDarwin.Code != 200 {
		t.Fatalf("darwin latest status = %d", latestDarwin.Code)
	}
	if latestDarwin.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("darwin latest cache = %q", latestDarwin.Header().Get("Cache-Control"))
	}
	downloadDarwin := httptest.NewRecorder()
	darwinRouter.ServeHTTP(downloadDarwin, authenticatedRequest("GET", "/api/v1/client/desktop/releases/20038/dmg"))
	if downloadDarwin.Code != 200 || downloadDarwin.Header().Get("X-Checksum-SHA256") != "darwin-checksum" {
		t.Fatalf("darwin download status = %d", downloadDarwin.Code)
	}
}
