package emby

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","UserData":{"PlaybackPositionTicks":420000000},"MediaSources":[{"Id":"source-1"}]}`))
		case "/Items/item-1/PlaybackInfo":
			_, _ = w.Write([]byte(`{"PlaySessionId":"play-session-1","MediaSources":[{"Id":"source-1","Path":"https://cdn.example/movie.mkv","Container":"mkv"}]}`))
		case "/Sessions/Playing", "/Sessions/Playing/Progress", "/Sessions/Playing/Stopped":
			var payload map[string]any
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload["ItemId"] != "item-1" || payload["MediaSourceId"] != "source-1" || payload["PlaySessionId"] != "play-session-1" {
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
	if media.URL != "https://cdn.example/movie.mkv" || media.Name != "Movie" || media.Session == nil || media.StartPositionMS != 42_000 {
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

func TestResolveEmbyItemAcceptsExternalRedirectWithoutExposingToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/Items/item-1":
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie"}`))
		case "/Items/item-1/PlaybackInfo":
			_, _ = w.Write([]byte(`{"MediaSources":[{"Id":"source-1","Path":"/private/movie.strm","Container":"mkv"}]}`))
		case "/Videos/item-1/stream.mkv":
			if request.Header.Get("Range") != "bytes=0-0" || request.Header.Get("X-Emby-Token") != "emby-key" || request.URL.Query().Get("EnableRedirection") != "true" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Location", "https://cdn.example/redirected.mkv?temporary=1")
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
	if media.URL != "https://cdn.example/redirected.mkv?temporary=1" || media.Session == nil {
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
	if !errors.Is(err, playback.ErrUnavailable) {
		t.Fatalf("error = %v", err)
	}
}
