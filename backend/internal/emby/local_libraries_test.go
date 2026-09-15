package emby

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"media-hub/backend/internal/playback"
)

func TestLibrariesListsLocalViewsAfterConfigured115(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != "test-key" || request.URL.Path != "/Users/user-1/Views" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{
				{"Id": "library-movies", "Name": "电影", "CollectionType": "movies"},
				{"Id": "library-shows", "Name": "电视剧", "CollectionType": "tvshows"},
				{"Id": "pt-movies", "Name": "英美电影", "CollectionType": "movies"},
				{"Id": "pt-shows", "Name": "美剧", "CollectionType": "tvshows"},
				{"Id": "metube", "Name": "Metube", "CollectionType": "movies"},
				{"Id": "live", "Name": "电视直播", "CollectionType": "livetv"},
				{"Id": "music", "Name": "音乐", "CollectionType": "music"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second, "user-1")
	client.Configure(RuntimeConfig{
		BaseURL: server.URL, APIKey: "test-key", UserID: "user-1",
		MovieLibraryID: "library-movies", SeriesLibraryID: "library-shows",
	})
	libraries, err := client.Libraries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(libraries) != 4 ||
		libraries[0].ID != "library-movies" || libraries[0].Name != "115电影" ||
		libraries[1].ID != "library-shows" || libraries[1].Name != "115电视剧" ||
		libraries[2].ID != "pt-movies" || libraries[2].Name != "英美电影" ||
		libraries[3].ID != "pt-shows" || libraries[3].Name != "美剧" {
		t.Fatalf("libraries=%#v", libraries)
	}
}

func TestBrowseAndSearchKeep115FilterAndExposeLocalLibraries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/Users/user-1/Views":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{
					{"Id": "library-1", "Name": "电影", "CollectionType": "movies"},
					{"Id": "pt-movies", "Name": "英美电影", "CollectionType": "movies"},
				},
			})
		case "/Items":
			query := request.URL.Query()
			switch query.Get("ParentId") {
			case "library-1":
				if query.Get("IncludeItemTypes") == "Episode" {
					_, _ = w.Write([]byte(`{"Items":[],"TotalRecordCount":0}`))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"Items": []map[string]any{
						{"Id": "cloud", "Name": "Cloud", "Type": "Movie", "Path": "/library/movie.strm", "MediaSources": []map[string]any{{"Path": "/library/movie.strm"}}},
						{"Id": "leftover", "Name": "Leftover", "Type": "Movie", "Path": "/volume1/media/movie.mkv"},
					},
					"TotalRecordCount": 2,
				})
			case "pt-movies":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"Items": []map[string]any{
						{"Id": "pt-1", "Name": "Local", "Type": "Movie", "Path": "/media/links/电影/英美电影/Local.mkv"},
					},
					"TotalRecordCount": 1,
				})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second, "user-1")
	client.Configure(RuntimeConfig{
		BaseURL: server.URL, APIKey: "test-key", UserID: "user-1", MovieLibraryID: "library-1",
	})

	cloud, err := client.BrowseItems(context.Background(), "library-1", 0, 10)
	if err != nil || cloud.Total != 1 || len(cloud.Items) != 1 || cloud.Items[0].ID != "cloud" {
		t.Fatalf("115 browse=%#v err=%v", cloud, err)
	}
	local, err := client.BrowseItems(context.Background(), "pt-movies", 0, 10)
	if err != nil || local.Total != 1 || local.Items[0].ID != "pt-1" {
		t.Fatalf("pt browse=%#v err=%v", local, err)
	}
	if _, err := client.BrowseItems(context.Background(), "metube", 0, 10); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("junk browse err=%v", err)
	}

	search, err := client.SearchItems(context.Background(), "Local", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	ids := make([]string, 0, len(search.Items))
	for _, item := range search.Items {
		ids = append(ids, item.ID)
	}
	if search.Total != 2 || ids[0] != "cloud" || ids[1] != "pt-1" {
		t.Fatalf("search=%#v", search)
	}
}

func TestItemDetailsAllowsLocalLibraryFilesAndHides115Leftovers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/Users/user-1/Views":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{
					{"Id": "library-1", "Name": "电影", "CollectionType": "movies"},
					{"Id": "pt-movies", "Name": "英美电影", "CollectionType": "movies"},
				},
			})
		case "/Users/user-1/Items/pt-1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Id": "pt-1", "Name": "Local", "Type": "Movie", "ParentId": "pt-movies",
				"Path": "/media/links/电影/英美电影/Local.mkv", "MediaSources": []map[string]any{{"Path": "/media/links/电影/英美电影/Local.mkv"}},
			})
		case "/Users/user-1/Items/leftover":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Id": "leftover", "Name": "Leftover", "Type": "Movie", "ParentId": "library-1",
				"Path": "/volume1/media/movie.mkv",
			})
		case "/System/Info":
			_, _ = w.Write([]byte(`{"Id":"server-1","ServerName":"Emby","Version":"4.9"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second, "user-1")
	client.Configure(RuntimeConfig{
		BaseURL: server.URL, APIKey: "test-key", UserID: "user-1", MovieLibraryID: "library-1",
	})
	detail, err := client.ItemDetails(context.Background(), "pt-1")
	if err != nil || detail.ID != "pt-1" || detail.Name != "Local" {
		t.Fatalf("detail=%#v err=%v", detail, err)
	}
	if _, err := client.ItemDetails(context.Background(), "leftover"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("leftover err=%v", err)
	}
}

func TestItemDetailsAllowsLocalFileWhenViewIdDiffersFromFolder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/Users/user-1/Items/pt-nested":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Id": "pt-nested", "Name": "Nested", "Type": "Movie", "ParentId": "folder-9",
				"Path": "/media/links/电影/英美电影/Nested.mkv",
			})
		case "/Items/pt-nested/Ancestors":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"Id": "folder-9", "Name": "Nested", "Type": "Folder"},
				{"Id": "collection-99", "Name": "英美电影", "Type": "CollectionFolder"},
			})
		case "/System/Info":
			_, _ = w.Write([]byte(`{"Id":"server-1","ServerName":"Emby","Version":"4.9"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second, "user-1")
	client.Configure(RuntimeConfig{
		BaseURL: server.URL, APIKey: "test-key", UserID: "user-1", MovieLibraryID: "library-1",
	})
	detail, err := client.ItemDetails(context.Background(), "pt-nested")
	if err != nil || detail.ID != "pt-nested" {
		t.Fatalf("detail=%#v err=%v", detail, err)
	}
}

func TestEpisodesIncludeLocalLibraryFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/Shows/series-1/Episodes" {
			if request.URL.Path == "/System/Info" {
				_, _ = w.Write([]byte(`{"Id":"server-1","ServerName":"Emby","Version":"4.9"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{
				{"Id": "ep-local", "Name": "第一集", "Type": "Episode", "ParentIndexNumber": 1, "IndexNumber": 1, "Path": "/media/links/电视剧/美剧/Show/S01E01.mkv"},
				{"Id": "ep-cloud", "Name": "云盘集", "Type": "Episode", "ParentIndexNumber": 1, "IndexNumber": 2, "Path": "/library/episode.strm"},
			},
		})
	}))
	defer server.Close()

	episodes, err := NewClient(server.URL, "test-key", time.Second, "user-1").Episodes(context.Background(), "series-1")
	if err != nil || len(episodes) != 2 || episodes[0].ID != "ep-local" || episodes[1].ID != "ep-cloud" {
		t.Fatalf("episodes=%#v err=%v", episodes, err)
	}
}

func TestDeletePreviewMarksLocalFilesAsNotCloudKept(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/Users/user-1/Views":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{{"Id": "pt-movies", "Name": "英美电影", "CollectionType": "movies"}},
			})
		case "/Users/user-1/Items/pt-1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Id": "pt-1", "Name": "Local", "Type": "Movie", "ParentId": "pt-movies",
				"Path": "/media/links/电影/Local.mkv",
			})
		case "/Items/pt-1/DeleteInfo":
			_ = json.NewEncoder(w).Encode(map[string]any{"Paths": []string{"/media/links/电影/Local.mkv"}, "LocalFileCount": 1})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "emby-key", time.Second, "user-1")
	client.Configure(RuntimeConfig{BaseURL: server.URL, APIKey: "emby-key", UserID: "user-1"})
	preview, err := client.DeletePreview(context.Background(), "pt-1")
	if err != nil || preview.ID != "pt-1" || preview.CloudKept || preview.FileCount != 1 {
		t.Fatalf("preview=%#v err=%v", preview, err)
	}
}

func TestResolveEmbyItemRejectsLocalLibraryFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/Users/user-1/Items/pt-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Id": "pt-1", "Name": "Local", "Type": "Movie",
			"Path":         "/media/links/电影/Local.mkv",
			"MediaSources": []map[string]any{{"Id": "source-1", "Path": "/media/links/电影/Local.mkv", "Container": "mkv"}},
		})
	}))
	defer server.Close()

	_, err := NewClient(server.URL, "emby-key", time.Second, "user-1").ResolveEmbyItem(
		context.Background(), playback.EmbyItemTarget{ItemID: "pt-1"}, "player-ua",
	)
	if !errors.Is(err, playback.ErrNotFound) && !errors.Is(err, playback.ErrUnavailable) {
		t.Fatalf("error=%v", err)
	}
}
