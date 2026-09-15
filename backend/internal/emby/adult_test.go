package emby

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAdultLibraryNameAndItemMarkers(t *testing.T) {
	if !isAdultLibraryName("成人电影") || !isAdultLibraryName("Adult Movies") || !isAdultLibraryName("av") || isAdultLibraryName("英美电影") {
		t.Fatal("library name markers")
	}
	if isAdultLibraryName("avatar") || isAdultLibraryName("travel") || isAdultLibraryName("brave") {
		t.Fatal("bare av matched inside unrelated names")
	}
	if !isAdultItem(baseItem{OfficialRating: "XXX"}) || !isAdultItem(baseItem{Genres: []string{"情色"}}) {
		t.Fatal("item markers")
	}
	if isAdultItem(baseItem{OfficialRating: "NC-17", Genres: []string{"剧情"}, Path: "/media/links/英美电影/Title.mkv"}) {
		t.Fatal("nc-17 mainstream item treated as adult")
	}
	if isAdultItem(baseItem{OfficialRating: "R", Genres: []string{"剧情"}, Path: "/media/links/英美电影/Title.mkv"}) {
		t.Fatal("mainstream item treated as adult")
	}
	if !isAdultItem(baseItem{Path: "/volume4/media/成人/Title.mkv"}) || !isAdultItem(baseItem{Path: "/media2/av/Title.mkv"}) {
		t.Fatal("adult path ignored")
	}
}

func TestLibrariesHidesAdultFoldersAndAddsAggregateTab(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/Users/user-1/Views":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{
					{"Id": "library-movies", "Name": "电影", "CollectionType": "movies"},
					{"Id": "pt-movies", "Name": "英美电影", "CollectionType": "movies"},
				},
			})
		case "/Library/MediaFolders":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{
					{"Id": "library-movies", "Name": "电影", "CollectionType": "movies"},
					{"Id": "pt-movies", "Name": "英美电影", "CollectionType": "movies"},
					{"Id": "adult-folder", "Name": "成人电影", "CollectionType": "movies"},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second, "user-1")
	client.Configure(RuntimeConfig{
		BaseURL: server.URL, APIKey: "test-key", UserID: "user-1", MovieLibraryID: "library-movies",
	})
	libraries, err := client.Libraries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(libraries))
	for _, library := range libraries {
		ids = append(ids, library.ID+"/"+library.Name)
	}
	if len(libraries) != 4 || libraries[0].ID != "library-movies" || libraries[1].ID != "pt-movies" ||
		libraries[2].ID != adultLibraryID || libraries[2].Name != adultLibraryName ||
		libraries[3].ID != "adult-folder" || libraries[3].ParentID != adultLibraryID || libraries[3].Name != "成人电影" {
		t.Fatalf("libraries=%v", ids)
	}
	for _, library := range libraries {
		if library.ID == "adult-folder" && library.ParentID != adultLibraryID {
			t.Fatalf("source adult folder leaked as a root tab: %#v", library)
		}
	}
	if got := client.cachedAdultFolderIDs(); len(got) != 1 || got[0] != "adult-folder" {
		t.Fatalf("adult folders=%v", got)
	}
}

func TestBrowseMovesAdultItemsIntoAggregateLibrary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/Users/user-1/Views":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{
					{"Id": "pt-movies", "Name": "英美电影", "CollectionType": "movies"},
				},
			})
		case "/Library/MediaFolders":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{
					{"Id": "adult-folder", "Name": "成人电影", "CollectionType": "movies"},
				},
			})
		case "/Items":
			query := request.URL.Query()
			switch query.Get("ParentId") {
			case "pt-movies":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"Items": []map[string]any{
						{"Id": "pt-1", "Name": "Local", "Type": "Movie", "Path": "/media/links/英美电影/Local.mkv"},
						{"Id": "pt-adult", "Name": "Tagged", "Type": "Movie", "OfficialRating": "XXX", "Path": "/media/links/英美电影/Tagged.mkv"},
					},
				})
			case "adult-folder":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"Items": []map[string]any{
						{"Id": "hidden-1", "Name": "Hidden", "Type": "Movie", "Path": "/volume4/media/成人/Hidden.mkv"},
						{"Id": "hidden-video", "Name": "Clip", "Type": "Video", "Path": "/media2/av/Clip.mp4"},
					},
				})
			case "":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"Items": []map[string]any{
						{"Id": "pt-adult", "Name": "Tagged", "Type": "Movie", "OfficialRating": "XXX", "Path": "/media/links/英美电影/Tagged.mkv"},
						{"Id": "hidden-1", "Name": "Hidden", "Type": "Movie", "Path": "/volume4/media/成人/Hidden.mkv"},
					},
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
	client.Configure(RuntimeConfig{BaseURL: server.URL, APIKey: "test-key", UserID: "user-1"})
	if _, err := client.Libraries(context.Background()); err != nil {
		t.Fatal(err)
	}
	regular, err := client.BrowseItems(context.Background(), "pt-movies", 0, 10)
	if err != nil || regular.Total != 1 || regular.Items[0].ID != "pt-1" {
		t.Fatalf("regular browse=%#v err=%v", regular, err)
	}
	adult, err := client.BrowseItems(context.Background(), adultLibraryID, 0, 10)
	if err != nil || adult.Total != 3 {
		t.Fatalf("adult browse=%#v err=%v", adult, err)
	}
	ids := map[string]bool{}
	for _, item := range adult.Items {
		ids[item.ID] = true
	}
	if !ids["hidden-1"] || !ids["pt-adult"] || !ids["hidden-video"] {
		t.Fatalf("adult ids=%v", ids)
	}
	group, err := client.BrowseItems(context.Background(), "adult-folder", 0, 10)
	if err != nil || group.Total != 2 {
		t.Fatalf("adult group=%#v err=%v", group, err)
	}
}

func TestAdultGroupsUseFirstLevelFolders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/Users/user-1/Views":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{
					{"Id": "library-movies", "Name": "电影", "CollectionType": "movies"},
				},
			})
		case "/Library/MediaFolders":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{
					{"Id": "adult-folder", "Name": "成人电影", "CollectionType": "movies"},
				},
			})
		case "/Items":
			query := request.URL.Query()
			if query.Get("Recursive") == "false" && query.Get("ParentId") == "adult-folder" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"Items": []map[string]any{
						{"Id": "group-jp", "Name": "日本", "Type": "Folder"},
						{"Id": "group-eu", "Name": "欧美", "Type": "Folder"},
					},
				})
				return
			}
			switch query.Get("ParentId") {
			case "group-jp":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"Items": []map[string]any{
						{"Id": "jp-1", "Name": "JP", "Type": "Movie", "Path": "/volume4/media/成人/日本/JP.mkv"},
					},
				})
			case "adult-folder":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"Items": []map[string]any{
						{"Id": "jp-1", "Name": "JP", "Type": "Movie", "Path": "/volume4/media/成人/日本/JP.mkv"},
						{"Id": "eu-1", "Name": "EU", "Type": "Movie", "Path": "/volume4/media/成人/欧美/EU.mkv"},
					},
				})
			default:
				_ = json.NewEncoder(w).Encode(map[string]any{"Items": []any{}})
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second, "user-1")
	client.Configure(RuntimeConfig{BaseURL: server.URL, APIKey: "test-key", UserID: "user-1"})
	libraries, err := client.Libraries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(libraries) != 4 || libraries[1].ID != adultLibraryID ||
		libraries[2].ID != "group-jp" || libraries[2].ParentID != adultLibraryID ||
		libraries[3].ID != "group-eu" || libraries[3].ParentID != adultLibraryID {
		t.Fatalf("libraries=%#v", libraries)
	}
	group, err := client.BrowseItems(context.Background(), "group-jp", 0, 10)
	if err != nil || group.Total != 1 || group.Items[0].ID != "jp-1" {
		t.Fatalf("group browse=%#v err=%v", group, err)
	}
}
