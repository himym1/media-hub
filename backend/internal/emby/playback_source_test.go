package emby

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"media-hub/backend/internal/playback"
)

func TestResolveEmbyItemAndReportPlaybackSession(t *testing.T) {
	var mutex sync.Mutex
	events := make([]string, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != "emby-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/Users/user-1/Items/item-1":
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/private/movie.strm","UserData":{"PlaybackPositionTicks":420000000},"MediaSources":[{"Id":"source-1","Path":"http://qms.local/115/url/video.mkv?pickcode=abcd1234","Container":"strm"}]}`))
		case "/Items/item-1/PlaybackInfo":
			w.WriteHeader(http.StatusGatewayTimeout)
		case "/Sessions/Playing", "/Sessions/Playing/Progress", "/Sessions/Playing/Stopped":
			var payload map[string]any
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload["ItemId"] != "item-1" || payload["MediaSourceId"] != "source-1" || payload["PlaySessionId"] == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			mutex.Lock()
			events = append(events, request.URL.Path)
			mutex.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "emby-key", time.Second, "user-1")
	media, err := client.ResolveEmbyItem(context.Background(), playback.EmbyItemTarget{ItemID: "item-1"}, "player-ua")
	if err != nil {
		t.Fatal(err)
	}
	if media.PickCode != "abcd1234" || media.URL != "" || media.Name != "Movie" || media.Session == nil || media.StartPositionMS != 42_000 {
		t.Fatalf("media = %#v", media)
	}
	for _, event := range []playback.SessionEvent{
		{Type: playback.SessionStarted},
		{Type: playback.SessionProgress, PositionMS: 15_000},
		{Type: playback.SessionStopped, PositionMS: 30_000, Paused: true},
	} {
		if err := media.Session.Reporter.ReportPlayback(context.Background(), media.Session.Reference, event); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"/Sessions/Playing", "/Sessions/Playing/Progress", "/Sessions/Playing/Stopped"}
	if len(events) != len(want) {
		t.Fatalf("events = %#v", events)
	}
	for index := range want {
		if events[index] != want[index] {
			t.Fatalf("events = %#v", events)
		}
	}
}

func TestResolveEmbyItemUsesItemPickCodeWithoutPlaybackInfo(t *testing.T) {
	playbackInfoHits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/Items/item-1":
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/media3/115-strm/movie.strm","UserData":{"PlaybackPositionTicks":120000000},"MediaSources":[{"Id":"source-1","Path":"https://media.example/115/url/video.mkv?pickcode=abcd1234","Container":"strm"}]}`))
		case "/Items/item-1/PlaybackInfo":
			playbackInfoHits++
			w.WriteHeader(http.StatusGatewayTimeout)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "emby-key", time.Second)
	media, err := client.ResolveEmbyItem(context.Background(), playback.EmbyItemTarget{ItemID: "item-1"}, "player-ua")
	if err != nil {
		t.Fatal(err)
	}
	if media.PickCode != "abcd1234" || media.URL != "" || media.Name != "Movie" || media.StartPositionMS != 12_000 || media.Session == nil {
		t.Fatalf("media = %#v", media)
	}
	if playbackInfoHits != 0 {
		t.Fatalf("PlaybackInfo hits = %d, want 0", playbackInfoHits)
	}
}

func TestResolveEmbyItemAcceptsExternalRedirectWithoutExposingToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/Items/item-1":
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/private/movie.strm","MediaSources":[{"Id":"source-1","Path":"/private/movie.strm","Container":"mkv"}]}`))
		case "/Items/item-1/PlaybackInfo":
			w.WriteHeader(http.StatusGatewayTimeout)
		case "/Videos/item-1/stream.mkv":
			if request.Header.Get("Range") != "" || request.Header.Get("X-Emby-Token") != "emby-key" || request.URL.Query().Get("EnableRedirection") != "true" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Location", "http://qms.local/115/url/video.mkv?pickcode=abcd1234")
			w.WriteHeader(http.StatusTemporaryRedirect)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "emby-key", time.Second)
	media, err := client.ResolveEmbyItem(context.Background(), playback.EmbyItemTarget{ItemID: "item-1"}, "player-ua")
	if err != nil {
		t.Fatal(err)
	}
	if media.PickCode != "abcd1234" || media.URL != "" || media.Session == nil {
		t.Fatalf("media = %#v", media)
	}
}

func TestResolveEmbyItemRejectsServerHostedStreamWithoutRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/Items/item-1":
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie"}`))
		case "/Items/item-1/PlaybackInfo":
			_, _ = w.Write([]byte(`{"MediaSources":[{"Id":"source-1","Path":"/private/movie.mkv"}]}`))
		default:
			w.WriteHeader(http.StatusPartialContent)
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, "emby-key", time.Second)
	_, err := client.ResolveEmbyItem(context.Background(), playback.EmbyItemTarget{ItemID: "item-1"}, "player-ua")
	if !errors.Is(err, playback.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveEmbyItemRejectsNonPlayableTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/Users/user-1/Items/folder-1":
			_, _ = w.Write([]byte(`{"Id":"folder-1","Name":"Season Folder","Type":"Folder","Path":"/library/show/season.strm"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "emby-key", time.Second, "user-1")
	_, err := client.ResolveEmbyItem(context.Background(), playback.EmbyItemTarget{ItemID: "folder-1"}, "player-ua")
	if !errors.Is(err, playback.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveEmbyItemFallsBackToSecondMediaSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != "emby-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/Users/user-1/Items/item-1":
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Episode 1","Type":"Episode","Path":"/library/episode.strm","MediaSources":[{"Id":"source-1","Path":"/library/broken.strm","Container":"strm"},{"Id":"source-2","Path":"https://qms.local/115/url/video.mkv?pickcode=abcd1234","Container":"strm"}]}`))
		case "/Items/item-1/PlaybackInfo":
			w.WriteHeader(http.StatusGatewayTimeout)
		case "/Videos/item-1/stream.strm":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "emby-key", time.Second, "user-1")
	media, err := client.ResolveEmbyItem(context.Background(), playback.EmbyItemTarget{ItemID: "item-1"}, "player-ua")
	if err != nil {
		t.Fatal(err)
	}
	if media.PickCode != "abcd1234" || media.Name != "Episode 1" {
		t.Fatalf("media = %#v", media)
	}
}

func TestResolveEmbyItemUsesImmutablePlaybackFacadeAfterCredentialReconfigure(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/Items/item-1":
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/strm/movie.strm","MediaSources":[{"Id":"source-1","Path":"/strm/movie.strm","Container":"mkv"}]}`))
		case "/Items/item-1/PlaybackInfo":
			w.WriteHeader(http.StatusGatewayTimeout)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer origin.Close()

	facadeRequests := 0
	facade := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		facadeRequests++
		if request.URL.Path != "/Videos/item-1/stream.mkv" || request.Header.Get("X-Emby-Token") != "new-key" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Location", "http://qms.local/115/url/video.mkv?pickcode=abcd1234")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer facade.Close()

	client := NewConfiguredClient(RuntimeConfig{BaseURL: origin.URL, APIKey: "old-key", PlaybackBaseURL: facade.URL}, time.Second)
	client.Configure(RuntimeConfig{BaseURL: origin.URL, APIKey: "new-key", PlaybackBaseURL: facade.URL})
	media, err := client.ResolveEmbyItem(context.Background(), playback.EmbyItemTarget{ItemID: "item-1"}, "player-ua")
	if err != nil {
		t.Fatal(err)
	}
	if media.PickCode != "abcd1234" || media.URL != "" || facadeRequests != 1 {
		t.Fatalf("media=%#v facadeRequests=%d", media, facadeRequests)
	}
}

func TestResolveEmbyItemPreservesPlaybackFacadeUnauthorized(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/Items/item-1":
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/strm/movie.strm","MediaSources":[{"Id":"source-1","Path":"/strm/movie.strm","Container":"strm"}]}`))
		case "/Items/item-1/PlaybackInfo":
			w.WriteHeader(http.StatusGatewayTimeout)
		}
	}))
	defer origin.Close()
	facade := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer facade.Close()

	client := NewConfiguredClient(RuntimeConfig{BaseURL: origin.URL, APIKey: "emby-key", PlaybackBaseURL: facade.URL}, time.Second)
	_, err := client.ResolveEmbyItem(context.Background(), playback.EmbyItemTarget{ItemID: "item-1"}, "player-ua")
	if !errors.Is(err, playback.ErrSourceUnauthorized) {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveEmbyItemExtractsPickCodeFromQMSRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/Items/item-1":
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/library/movie.strm"}`))
		case "/Items/item-1/PlaybackInfo":
			w.WriteHeader(http.StatusGatewayTimeout)
		case "/Videos/item-1/stream.strm":
			w.Header().Set("Location", "http://qms.local/115/url/video.mkv?pickcode=abcd1234")
			w.WriteHeader(http.StatusTemporaryRedirect)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, "emby-key", time.Second)
	media, err := client.ResolveEmbyItem(context.Background(), playback.EmbyItemTarget{ItemID: "item-1"}, "player-ua")
	if err != nil {
		t.Fatal(err)
	}
	if media.PickCode != "abcd1234" || media.URL != "" {
		t.Fatalf("media=%#v", media)
	}
}

func TestResolveEmbyItemUses115CDNRedirectWithoutPickCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/Items/item-1":
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/library/movie.strm"}`))
		case "/Items/item-1/PlaybackInfo":
			w.WriteHeader(http.StatusGatewayTimeout)
		case "/Videos/item-1/stream.strm":
			w.Header().Set("Location", "https://cdnfhnfile.115.com/video.mkv?t=1")
			w.WriteHeader(http.StatusTemporaryRedirect)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, "emby-key", time.Second)
	media, err := client.ResolveEmbyItem(context.Background(), playback.EmbyItemTarget{ItemID: "item-1"}, "player-ua")
	if err != nil {
		t.Fatal(err)
	}
	if media.URL != "https://cdnfhnfile.115.com/video.mkv?t=1" || media.PickCode != "" {
		t.Fatalf("media=%#v", media)
	}
}

func TestResolveEmbyItemFollowsUnsignedSecondHopForPickCode(t *testing.T) {
	var hopAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		hopAuth = request.Header.Get("X-Emby-Token")
		w.Header().Set("Location", "http://qms.local/115/url/video.mkv?pickcode=abcd1234")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer upstream.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/Items/item-1":
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/library/movie.strm"}`))
		case "/Items/item-1/PlaybackInfo":
			w.WriteHeader(http.StatusGatewayTimeout)
		case "/Videos/item-1/stream.strm":
			w.Header().Set("Location", upstream.URL+"/next")
			w.WriteHeader(http.StatusTemporaryRedirect)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, "emby-key", time.Second)
	media, err := client.ResolveEmbyItem(context.Background(), playback.EmbyItemTarget{ItemID: "item-1"}, "player-ua")
	if err != nil {
		t.Fatal(err)
	}
	if media.PickCode != "abcd1234" || hopAuth != "" {
		t.Fatalf("media=%#v hopAuth=%q", media, hopAuth)
	}
}

func TestResolveEmbyItemReadsPickCodeFromStrmScheme(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/Items/item-1":
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/library/movie.strm"}`))
		case "/Items/item-1/PlaybackInfo":
			w.WriteHeader(http.StatusGatewayTimeout)
		case "/Items/item-1/Download":
			_, _ = w.Write([]byte("115://abcd1234\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, "emby-key", time.Second)
	media, err := client.ResolveEmbyItem(context.Background(), playback.EmbyItemTarget{ItemID: "item-1"}, "player-ua")
	if err != nil {
		t.Fatal(err)
	}
	if media.PickCode != "abcd1234" {
		t.Fatalf("media=%#v", media)
	}
}

func TestLocalStreamURLUsesLibraryHostWithoutPlaybackFacade(t *testing.T) {
	client := NewConfiguredClient(RuntimeConfig{
		BaseURL: "http://192.168.1.8:8096", APIKey: "emby-key", PlaybackBaseURL: "http://qms.local:8095",
	}, time.Second)
	location, err := client.LocalStreamURL(playback.LocalRef{ItemID: "pt-1", MediaSourceID: "source-1", Container: "mkv"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(location)
	if err != nil || parsed.Scheme != "http" || parsed.Host != "192.168.1.8:8096" || parsed.Path != "/Videos/pt-1/stream.mkv" {
		t.Fatalf("location=%q", location)
	}
	if parsed.Query().Get("Static") != "true" || parsed.Query().Get("MediaSourceId") != "source-1" || parsed.Query().Get("api_key") != "emby-key" {
		t.Fatalf("query=%q", parsed.RawQuery)
	}
}

func TestResolveEmbyItemRejectsHTTPSWithoutPickCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/Items/item-1":
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/library/other.strm","MediaSources":[{"Path":"https://other.example/video.m3u8"}]}`))
		case "/Items/item-1/PlaybackInfo":
			_, _ = w.Write([]byte(`{"MediaSources":[{"Id":"source-1","Path":"https://other.example/video.m3u8","Container":"strm"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, "emby-key", time.Second)
	_, err := client.ResolveEmbyItem(context.Background(), playback.EmbyItemTarget{ItemID: "item-1"}, "player-ua")
	if !errors.Is(err, playback.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}
