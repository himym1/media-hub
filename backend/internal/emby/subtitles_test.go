package emby

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSearchAndDownloadRemoteSubtitles(t *testing.T) {
	var searchedPaths []string
	var downloadedPath string
	var refreshed bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/Items/item-1/RemoteSearch/Subtitles/"):
			searchedPaths = append(searchedPaths, request.URL.Path)
			lang := strings.TrimPrefix(request.URL.Path, "/Items/item-1/RemoteSearch/Subtitles/")
			if lang == "chi" {
				_, _ = w.Write([]byte(`[
					{"Id":"opensubtitles-1","Name":"Movie.chi.srt","ThreeLetterISOLanguageName":"chi","Format":"srt","ProviderName":"Open Subtitles","DownloadCount":12,"IsHashMatch":true},
					{"Id":"opensubtitles/nested","Name":"Movie.zh.ass","ThreeLetterISOLanguageName":"chi","Format":"ass","ProviderName":"Open Subtitles","DownloadCount":3}
				]`))
				return
			}
			_, _ = w.Write([]byte(`[]`))
		case request.Method == http.MethodPost && strings.HasPrefix(request.URL.Path, "/Items/item-1/RemoteSearch/Subtitles/"):
			downloadedPath = request.URL.EscapedPath()
			w.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodPost && request.URL.Path == "/Items/item-1/Refresh":
			refreshed = true
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second, "user-1")
	client.Configure(RuntimeConfig{BaseURL: server.URL, APIKey: "test-key", UserID: "user-1"})

	items, err := client.SearchRemoteSubtitles(context.Background(), "item-1", "zh")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(items) != 2 || items[0].ID != "opensubtitles-1" || !items[0].IsHashMatch || items[1].Format != "ass" {
		t.Fatalf("unexpected search result: %#v", items)
	}
	if len(searchedPaths) != 1 || searchedPaths[0] != "/Items/item-1/RemoteSearch/Subtitles/chi" {
		t.Fatalf("unexpected search paths: %#v", searchedPaths)
	}

	if err := client.DownloadRemoteSubtitle(context.Background(), "item-1", "opensubtitles/nested"); err != nil {
		t.Fatalf("download: %v", err)
	}
	if downloadedPath != "/Items/item-1/RemoteSearch/Subtitles/opensubtitles%2Fnested" {
		t.Fatalf("downloaded path = %q", downloadedPath)
	}
	if !refreshed {
		t.Fatal("expected refresh after download")
	}
}

func TestSearchRemoteSubtitlesFallsBackLanguage(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		attempts++
		lang := strings.TrimPrefix(request.URL.Path, "/Items/item-2/RemoteSearch/Subtitles/")
		if lang == "chi" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if lang == "zh" {
			_, _ = w.Write([]byte(`[{"Id":"sub-zh","Name":"zh.srt","Format":"srt","ProviderName":"Open Subtitles"}]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second, "")
	client.Configure(RuntimeConfig{BaseURL: server.URL, APIKey: "test-key"})
	items, err := client.SearchRemoteSubtitles(context.Background(), "item-2", "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(items) != 1 || items[0].ID != "sub-zh" || attempts != 2 {
		t.Fatalf("items=%#v attempts=%d", items, attempts)
	}
}
