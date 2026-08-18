package emby

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
			_, _ = w.Write([]byte(`{"Items":[{"Id":"library-1","Name":"电影","CollectionType":"movies"},{"Id":"library-music","Name":"音乐","CollectionType":"music"},{"Id":"library-empty","Name":"本地电影","CollectionType":"movies"}],"TotalRecordCount":3}`))
		case "/Users/user-1/Items/item-1":
			query := request.URL.Query()
			if query.Get("UserId") != "" || !strings.Contains(query.Get("Fields"), "Overview") {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"范海辛","OriginalTitle":"Van Helsing","Overview":"Monster hunter","Type":"Movie","ProductionYear":2004,"Path":"/private/movie.strm","ProviderIds":{"Tmdb":"7131"},"CommunityRating":7.2,"RunTimeTicks":79200000000,"Genres":["Action"],"MediaSources":[{"Id":"source-1","Path":"/private/movie.strm","Container":"strm"}]}`))
		case "/System/Info":
			_, _ = w.Write([]byte(`{"Id":"server-1","ServerName":"Test Emby","Version":"4.9.3"}`))
		case "/Items":
			if request.URL.Query().Get("UserId") != "user-1" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			query := request.URL.Query()
			switch {
			case query.Get("ParentId") != "":
				if query.Get("IncludeItemTypes") == "Episode" {
					_, _ = w.Write([]byte(`{"Items":[],"TotalRecordCount":0}`))
					return
				}
				if query.Get("IncludeItemTypes") == "Movie" {
					if query.Get("ParentId") != "library-1" {
						_, _ = w.Write([]byte(`{"Items":[],"TotalRecordCount":0}`))
						return
					}
					_, _ = w.Write([]byte(`{"Items":[{"Id":"item-2","Name":"Library Movie","Type":"Movie","Path":"/library/movie.strm","MediaSources":[{"Path":"/library/movie.strm"}]}],"TotalRecordCount":1}`))
					return
				}
				if query.Get("ParentId") != "library-1" || query.Get("StartIndex") != "0" || query.Get("Limit") != "10000" || !strings.Contains(query.Get("Fields"), "MediaSources") {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				_, _ = w.Write([]byte(`{"Items":[{"Id":"item-2","Name":"Library Movie","Type":"Movie","ProductionYear":2025,"Path":"/library/movie.strm","ProviderIds":{"Tmdb":"999"},"MediaSources":[{"Path":"/library/movie.strm"}],"UserData":{"PlaybackPositionTicks":25000000000}}],"TotalRecordCount":21}`))
			case query.Get("AnyProviderIdEquals") != "":
				providerQueries++
				if query.Get("AnyProviderIdEquals") != "Tmdb.7131" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				_, _ = w.Write([]byte(`{"Items":[{"Id":"item-1","Name":"范海辛","Type":"Movie","ProductionYear":2004,"Path":"/private/movie.mkv","ProviderIds":{"Tmdb":"7131"}}],"TotalRecordCount":1}`))
			default:
				titleQueries++
				_, _ = w.Write([]byte(`{"Items":[{"Id":"item-1","Name":"范海辛","Type":"Movie","ProductionYear":2004,"Path":"/private/movie.strm","ProviderIds":{"Tmdb":"7131"},"MediaSources":[{"Path":"/private/movie.strm"}]}],"TotalRecordCount":1}`))
			}
		case "/Items/library-1/Refresh", "/Items/item-1/Refresh":
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
	browse, err := client.BrowseItems(context.Background(), "library-1", 0, 10)
	if err != nil || browse.Total != 1 || len(browse.Items) != 1 || browse.Items[0].ID != "item-2" || browse.Items[0].PlaybackPositionMS != 2_500_000 {
		t.Fatalf("browse items: result=%#v err=%v", browse, err)
	}
	detail, err := client.ItemDetails(context.Background(), "item-1")
	if err != nil || detail.MediaSourceCount != 1 || detail.RuntimeMinutes != 132 || detail.ProviderIDs["Tmdb"] != "7131" {
		t.Fatalf("item details: detail=%#v err=%v", detail, err)
	}
	if strings.Contains(detail.ExternalURL, "test-key") || !strings.Contains(detail.ExternalURL, "item?id=item-1") {
		t.Fatalf("unsafe or invalid external URL: %q", detail.ExternalURL)
	}
	if detail.AppURL != "emby://items/server-1/item-1" {
		t.Fatalf("invalid app URL: %q", detail.AppURL)
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
	if err := client.RefreshItem(context.Background(), "item-1"); err != nil {
		t.Fatalf("refresh item: %v", err)
	}
	ready, err := client.PlaybackReady(context.Background(), "item-1")
	if err != nil || !ready {
		t.Fatalf("playback ready=%v err=%v", ready, err)
	}
}

func TestItemDetailsWithoutUserUsesDirectEndpointAndMapsOnlyNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/Items/item-1":
			if request.URL.Query().Get("UserId") != "" || !strings.Contains(request.URL.Query().Get("Fields"), "MediaSources") {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/private/movie.strm","MediaSources":[{"Path":"/private/movie.strm"}],"UserData":{"PlaybackPositionTicks":650000000,"Played":false}}`))
		case "/Items/missing":
			w.WriteHeader(http.StatusNotFound)
		case "/Items/failure":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second)
	detail, err := client.ItemDetails(context.Background(), "item-1")
	if err != nil || detail.ID != "item-1" || detail.AppURL != "" || detail.PlaybackPositionMS != 65_000 || detail.Played {
		t.Fatalf("item details: detail=%#v err=%v", detail, err)
	}
	if _, err := client.ItemDetails(context.Background(), "missing"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("missing item error=%v", err)
	}
	if _, err := client.ItemDetails(context.Background(), "failure"); !errors.Is(err, ErrUpstreamResponse) {
		t.Fatalf("upstream failure error=%v", err)
	}
}

func TestItemWebURLDropsConfigurationQueryAndRejectsNonHTTP(t *testing.T) {
	value, err := itemWebURL("https://emby.example/base?api_key=secret#old", "item-1")
	if err != nil || strings.Contains(value, "secret") || value != "https://emby.example/base/web/index.html#!/item?id=item-1" {
		t.Fatalf("item web URL=%q err=%v", value, err)
	}
	if _, err := itemWebURL("javascript:alert(1)", "item-1"); !errors.Is(err, ErrUpstreamResponse) {
		t.Fatalf("non-http URL error=%v", err)
	}
}

func TestItemAppURLAcceptsOnlyBoundedEmbyIdentifiers(t *testing.T) {
	if value := itemAppURL("server-1", "item_1"); value != "emby://items/server-1/item_1" {
		t.Fatalf("item app URL=%q", value)
	}
	for _, input := range [][2]string{{"", "item-1"}, {"server/other", "item-1"}, {"server-1", "../item"}, {strings.Repeat("a", 129), "item-1"}} {
		if value := itemAppURL(input[0], input[1]); value != "" {
			t.Fatalf("unsafe item app URL=%q for %#v", value, input)
		}
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
