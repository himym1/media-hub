package emby

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"media-hub/backend/internal/playback"
)

func TestSharedLibrariesBrowseAndPlayback(t *testing.T) {
	t.Parallel()
	var authCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/Users/AuthenticateByName":
			authCalls++
			auth := r.Header.Get("X-Emby-Authorization")
			if !strings.Contains(auth, `Client="Emby Web"`) {
				http.Error(w, "请使用群公告中允许的客户端进行访问", http.StatusForbidden)
				return
			}
			if !strings.HasPrefix(r.Header.Get("User-Agent"), "Emby Web/") {
				http.Error(w, "请使用群公告中允许的客户端进行访问", http.StatusForbidden)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"AccessToken": "shared-token",
				"User":        map[string]any{"Id": "user-1", "Name": "himym"},
			})
		case r.URL.Path == "/Users/user-1/Views":
			if r.Header.Get("X-Emby-Token") != "shared-token" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if !strings.Contains(r.Header.Get("X-Emby-Authorization"), `Client="Emby Web"`) {
				http.Error(w, "请使用群公告中允许的客户端进行访问", http.StatusForbidden)
				return
			}
			if !strings.HasPrefix(r.Header.Get("User-Agent"), "Emby Web/") {
				http.Error(w, "请使用群公告中允许的客户端进行访问", http.StatusForbidden)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{{"Id": "lib-movies", "Name": "电影", "CollectionType": "movies"}},
			})
		case r.URL.Path == "/Items" && r.URL.Query().Get("ParentId") == "lib-movies":
			if !strings.HasPrefix(r.Header.Get("User-Agent"), "Emby Web/") {
				http.Error(w, "请使用群公告中允许的客户端进行访问", http.StatusForbidden)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{{
					"Id": "movie-1", "Name": "新片", "Type": "Movie", "ProductionYear": 2026,
					"MediaSources": []map[string]any{{"Id": "src-1", "Container": "mkv"}},
				}},
				"TotalRecordCount": 1,
			})
		case r.URL.Path == "/Users/user-1/Items/movie-1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Id": "movie-1", "Name": "新片", "Type": "Movie",
				"MediaSources": []map[string]any{{"Id": "src-1", "Container": "mkv", "SupportsDirectPlay": true}},
				"UserData":     map[string]any{"PlaybackPositionTicks": 0},
			})
		case strings.HasPrefix(r.URL.Path, "/Videos/movie-1/stream"):
			w.Header().Set("Content-Type", "video/x-matroska")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte{0x1a, 0x45})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client := NewConfiguredClient(RuntimeConfig{
		BaseURL: server.URL, Username: "himym", Password: "secret", Shared: true,
	}, server.Client().Timeout)
	// httptest uses http; force base through Configure after rewriting to keep test simple by allowing http in endpointURL.
	client.Configure(RuntimeConfig{BaseURL: server.URL, Username: "himym", Password: "secret", Shared: true})

	libraries, err := client.sharedLibraries(context.Background())
	if err != nil {
		t.Fatalf("libraries: %v", err)
	}
	if len(libraries) != 1 || libraries[0].ID != "r_lib-movies" || libraries[0].Name != "共享/电影" {
		t.Fatalf("libraries = %#v", libraries)
	}
	if authCalls != 1 {
		t.Fatalf("auth calls = %d", authCalls)
	}

	browse, err := client.sharedBrowseItems(context.Background(), "r_lib-movies", 0, 24, "")
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if browse.Total != 1 || browse.Items[0].ID != "r_movie-1" {
		t.Fatalf("browse = %#v", browse)
	}

	// Stream URL builder requires https; skip full ResolveSharedItem against httptest.
	media, err := client.ResolveSharedItem(context.Background(), playback.EmbyItemTarget{ItemID: "r_movie-1"}, "Mozilla/5.0")
	if err == nil && media.URL != "" {
		t.Fatalf("expected https-only stream rejection or empty against http test server, got %#v", media)
	}
}

func TestProbeSharedStreamAcceptsCDNRedirect(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Videos/movie-1/stream.mkv" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Range") != "bytes=0-0" || r.Header.Get("X-Emby-Token") != "shared-token" {
			http.Error(w, "bad probe", http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, "https://cdn.example/video.mkv?temporary=1", http.StatusFound)
	}))
	t.Cleanup(server.Close)
	client := NewConfiguredClient(RuntimeConfig{BaseURL: server.URL, Username: "u", Password: "p", Shared: true}, 0)
	if err := client.probeSharedStream(context.Background(), server.URL+"/Videos/movie-1/stream.mkv", "shared-token"); err != nil {
		t.Fatalf("redirect probe: %v", err)
	}
}

func TestHubRoutesSharedIDs(t *testing.T) {
	t.Parallel()
	localServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Library/MediaFolders" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{{"Id": "local-movies", "Name": "电影", "CollectionType": "movies"}},
		})
	}))
	t.Cleanup(localServer.Close)
	local := NewConfiguredClient(RuntimeConfig{
		BaseURL: localServer.URL, APIKey: "key", MovieLibraryID: "local-movies",
	}, 0)
	sharedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Users/AuthenticateByName":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"AccessToken": "tok", "User": map[string]any{"Id": "u1", "Name": "x"},
			})
		case "/Users/u1/Views":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{{"Id": "remote", "Name": "电影", "CollectionType": "movies"}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(sharedServer.Close)
	shared := NewConfiguredClient(RuntimeConfig{
		BaseURL: sharedServer.URL, Username: "u", Password: "p", Shared: true,
	}, 0)
	hub := NewHub(local, shared)
	libraries, err := hub.Libraries(context.Background())
	if err != nil {
		t.Fatalf("libraries: %v", err)
	}
	var sawLocal, sawShared bool
	for _, library := range libraries {
		if library.ID == "local-movies" {
			sawLocal = true
		}
		if library.ID == "r_remote" {
			sawShared = true
		}
	}
	if !sawLocal || !sawShared {
		t.Fatalf("libraries = %#v", libraries)
	}
}

func TestSharedBrowseItemsHonorsYearSort(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Users/AuthenticateByName":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"AccessToken": "tok", "User": map[string]any{"Id": "u1", "Name": "x"},
			})
		case "/Items":
			if r.URL.Query().Get("ParentId") != "lib-movies" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{
					{"Id": "old", "Name": "老片", "Type": "Movie", "ProductionYear": 1999, "DateCreated": "2020-01-01T00:00:00.0000000Z"},
					{"Id": "new", "Name": "新片", "Type": "Movie", "ProductionYear": 2026, "DateCreated": "2026-01-01T00:00:00.0000000Z"},
				},
				"TotalRecordCount": 2,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	client := NewConfiguredClient(RuntimeConfig{BaseURL: server.URL, Username: "u", Password: "p", Shared: true}, 0)
	browse, err := client.sharedBrowseItems(context.Background(), "r_lib-movies", 0, 24, "year-desc")
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if len(browse.Items) != 2 || browse.Items[0].ID != "r_new" || browse.Items[1].ID != "r_old" {
		t.Fatalf("year-desc = %#v", browse.Items)
	}
}

func TestSharedEpisodesSortBySeasonAndIndex(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Users/AuthenticateByName":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"AccessToken": "tok", "User": map[string]any{"Id": "u1", "Name": "x"},
			})
		case "/Items":
			if r.URL.Query().Get("ParentId") != "series-1" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{
					{"Id": "e2", "Name": "Zebra", "Type": "Episode", "ParentIndexNumber": 1, "IndexNumber": 2, "Path": "/lib/S01E02.mkv"},
					{"Id": "e1", "Name": "Alpha", "Type": "Episode", "ParentIndexNumber": 1, "IndexNumber": 1, "Path": "/lib/S01E01.mkv"},
					{"Id": "e3", "Name": "Yearly", "Type": "Episode", "ParentIndexNumber": 2016, "IndexNumber": 3, "Path": "/lib/Show 2016 E03.mkv"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	client := NewConfiguredClient(RuntimeConfig{BaseURL: server.URL, Username: "u", Password: "p", Shared: true}, 0)
	episodes, err := client.sharedEpisodes(context.Background(), "r_series-1")
	if err != nil {
		t.Fatalf("episodes: %v", err)
	}
	if len(episodes) != 3 {
		t.Fatalf("episodes = %#v", episodes)
	}
	if episodes[0].ID != "r_e1" || episodes[0].Episode != 1 || episodes[1].ID != "r_e2" || episodes[2].ID != "r_e3" {
		t.Fatalf("order = %#v", episodes)
	}
	if episodes[2].Season != 1 || episodes[2].Episode != 3 {
		t.Fatalf("normalized year-season = %#v", episodes[2])
	}
}
