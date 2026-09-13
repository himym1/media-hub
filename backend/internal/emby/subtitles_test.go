package emby

import (
	"context"
	"encoding/json"
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
		if !strings.HasPrefix(request.URL.Path, "/Items/item-2/RemoteSearch/Subtitles/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
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

func TestSearchRemoteSubtitlesChiDoesNotFallBackToZh(t *testing.T) {
	var languages []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.URL.Path, "/Items/item-3/RemoteSearch/Subtitles/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		languages = append(languages, strings.TrimPrefix(request.URL.Path, "/Items/item-3/RemoteSearch/Subtitles/"))
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second, "")
	items, err := client.SearchRemoteSubtitles(context.Background(), "item-3", "chi")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("items=%#v", items)
	}
	if len(languages) != 1 || languages[0] != "chi" {
		t.Fatalf("languages=%#v", languages)
	}
}

func TestSearchRemoteSubtitlesCorrectsYearSeasonBeforeSearch(t *testing.T) {
	var updated bool
	var searched bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/Items/item-4":
			_, _ = w.Write([]byte(`{
				"Id":"item-4","Name":"E01","Type":"Episode",
				"ParentIndexNumber":2016,"IndexNumber":1,
				"Path":"/library/Show 2016 E01.strm"
			}`))
		case request.Method == http.MethodPost && request.URL.Path == "/Items/item-4":
			var payload map[string]any
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Errorf("decode update: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if jsonNumber(payload["ParentIndexNumber"]) != 1 || jsonNumber(payload["IndexNumber"]) != 1 {
				t.Errorf("updated numbering = season %#v episode %#v", payload["ParentIndexNumber"], payload["IndexNumber"])
			}
			updated = true
			w.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodGet && request.URL.Path == "/Items/item-4/RemoteSearch/Subtitles/chi":
			if !updated {
				t.Error("expected season correction before subtitle search")
			}
			searched = true
			_, _ = w.Write([]byte(`[{"Id":"sub-1","Name":"chi.srt","Format":"srt"}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second, "")
	items, err := client.SearchRemoteSubtitles(context.Background(), "item-4", "chi")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !updated || !searched || len(items) != 1 || items[0].ID != "sub-1" {
		t.Fatalf("updated=%v searched=%v items=%#v", updated, searched, items)
	}
}

func TestNormalizeEpisodeNumbering(t *testing.T) {
	t.Parallel()
	cases := []struct {
		itemType string
		season   int
		episode  int
		path     string
		wantS    int
		wantE    int
		changed  bool
	}{
		{itemType: "Episode", season: 2016, episode: 1, path: "/library/Show 2016 E01.strm", wantS: 1, wantE: 1, changed: true},
		{itemType: "Episode", season: 2016, episode: 9, path: "/library/Show.S02E09.mkv", wantS: 2, wantE: 9, changed: true},
		{itemType: "Episode", season: 1, episode: 3, path: "/library/Show S01E03.mkv", wantS: 1, wantE: 3, changed: false},
		{itemType: "Movie", season: 2016, episode: 0, path: "/library/Movie 2016.mkv", wantS: 2016, wantE: 0, changed: false},
	}
	for _, tc := range cases {
		season, episode, changed := normalizeEpisodeNumbering(tc.itemType, tc.season, tc.episode, tc.path)
		if season != tc.wantS || episode != tc.wantE || changed != tc.changed {
			t.Fatalf("%s season %d episode %d path %q => %d %d %v, want %d %d %v",
				tc.itemType, tc.season, tc.episode, tc.path, season, episode, changed, tc.wantS, tc.wantE, tc.changed)
		}
	}
}

func TestNewConfiguredClientUsesLongerSubtitleTimeout(t *testing.T) {
	t.Parallel()
	client := NewConfiguredClient(RuntimeConfig{BaseURL: "http://emby.local", APIKey: "key"}, time.Second)
	if client.subtitleClient == nil || client.subtitleClient.Timeout != defaultSubtitleTimeout {
		t.Fatalf("subtitle timeout = %v", client.subtitleClient)
	}
	if client.client.Timeout != time.Second {
		t.Fatalf("probe timeout = %s", client.client.Timeout)
	}
	if client.libraryClient == nil || client.libraryClient.Timeout != defaultLibraryTimeout {
		t.Fatalf("library timeout = %v", client.libraryClient)
	}
}
