package emby

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientReadsLibrariesAndSearchesWithoutExposingPaths(t *testing.T) {
	providerQueries := 0
	titleQueries := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/Library/MediaFolders":
			_, _ = w.Write([]byte(`{"Items":[{"Id":"library-1","Name":"电影","CollectionType":"movies"}],"TotalRecordCount":1}`))
		case "/Items":
			if providerID := request.URL.Query().Get("AnyProviderIdEquals"); providerID != "" {
				providerQueries++
				if providerID != "Tmdb.7131" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
			} else {
				titleQueries++
			}
			_, _ = w.Write([]byte(`{"Items":[{"Id":"item-1","Name":"范海辛","Type":"Movie","ProductionYear":2004,"Path":"/private/movie.mkv","ProviderIds":{"Tmdb":"7131"}}],"TotalRecordCount":1}`))
		case "/Items/library-1/Refresh":
			if request.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case "/Items/item-1/PlaybackInfo":
			if request.Method != http.MethodPost || request.URL.Query().Get("UserId") != "user-1" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"MediaSources":[{"Id":"source-1","Path":"/private/movie.mkv"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second, "user-1")
	libraries, err := client.Libraries(context.Background())
	if err != nil {
		t.Fatalf("read libraries: %v", err)
	}
	if len(libraries) != 1 || libraries[0].CollectionType != "movies" {
		t.Fatalf("unexpected libraries: %#v", libraries)
	}

	result, err := client.SearchItems(context.Background(), "范海辛", 10)
	if err != nil {
		t.Fatalf("search items: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].ProviderIDs["Tmdb"] != "7131" {
		t.Fatalf("unexpected result: %#v", result)
	}

	item, found, err := client.FindIndexedItem(context.Background(), "Van Helsing", "movie", 2004, "7131")
	if err != nil || !found || item.ID != "item-1" {
		t.Fatalf("find indexed item: item=%#v found=%v err=%v", item, found, err)
	}
	if providerQueries != 1 || titleQueries != 1 {
		t.Fatalf("provider queries=%d title queries=%d", providerQueries, titleQueries)
	}
	if err := client.RefreshLibrary(context.Background(), "library-1"); err != nil {
		t.Fatalf("refresh library: %v", err)
	}
	ready, err := client.PlaybackReady(context.Background(), "item-1")
	if err != nil || !ready {
		t.Fatalf("playback ready=%v err=%v", ready, err)
	}
}

func TestFindPlayableItemRequiresEveryEpisodeInRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/Shows/series-1/Episodes":
			_, _ = w.Write([]byte(`{"Items":[{"Id":"episode-1","Type":"Episode","ParentIndexNumber":1,"IndexNumber":1,"MediaSources":[{"Id":"source-1"}]},{"Id":"episode-2","Type":"Episode","ParentIndexNumber":1,"IndexNumber":2,"MediaSources":[{"Id":"source-2"}]}],"TotalRecordCount":2}`))
		case request.URL.Path == "/Items":
			_, _ = w.Write([]byte(`{"Items":[{"Id":"series-1","Name":"Example","Type":"Series","ProductionYear":2024,"ProviderIds":{"Tmdb":"123"}}],"TotalRecordCount":1}`))
		case request.URL.Path == "/Items/episode-1/PlaybackInfo", request.URL.Path == "/Items/episode-2/PlaybackInfo":
			_, _ = w.Write([]byte(`{"MediaSources":[{"Id":"source"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second, "")
	item, found, err := client.FindPlayableItem(context.Background(), "Example", "series", 2024, "123", 1, 1, 2)
	if err != nil || !found || item.ID != "series-1" {
		t.Fatalf("find playable range: item=%#v found=%v err=%v", item, found, err)
	}
	_, found, err = client.FindPlayableItem(context.Background(), "Example", "series", 2024, "123", 1, 1, 3)
	if err != nil || found {
		t.Fatalf("missing episode range found=%v err=%v", found, err)
	}
}
