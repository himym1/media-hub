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

	download := httptest.NewRecorder()
	router.ServeHTTP(download, authenticatedRequest("GET", "/api/v1/client/android/releases/6000/apk"))
	if download.Code != 200 || download.Body.String() != "apk" {
		t.Fatalf("download status = %d body = %q", download.Code, download.Body.String())
	}
	if download.Header().Get("X-Checksum-SHA256") != "checksum" {
		t.Fatal("missing checksum header")
	}
}
