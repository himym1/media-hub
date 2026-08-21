package emby

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEpisodesExposeUserPlaybackStateWithoutCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != "emby-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/Shows/series-1/Episodes":
			if request.URL.Query().Get("UserId") != "user-1" || !strings.Contains(request.URL.Query().Get("Fields"), "UserData") {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"Items":[{"Id":"episode-1","Name":"Episode 1","Type":"Episode","ParentIndexNumber":1,"IndexNumber":1,"Path":"/library/episode.strm","MediaSources":[{"Id":"source-1","Path":"/library/episode.strm"}],"UserData":{"PlaybackPositionTicks":650000000,"Played":true}}]}`))
		case "/System/Info":
			_, _ = w.Write([]byte(`{"Id":"server-1","ServerName":"Emby","Version":"4.9"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "emby-key", time.Second, "user-1")
	episodes, err := client.Episodes(context.Background(), "series-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) != 1 || episodes[0].PlaybackPositionMS != 65_000 || !episodes[0].Played {
		t.Fatalf("episodes = %#v", episodes)
	}
	if episodes[0].AppURL != "emby://items/server-1/episode-1" || strings.Contains(episodes[0].ExternalURL, "emby-key") {
		t.Fatalf("episode URLs = %#v", episodes[0])
	}
}

func TestPrimaryImagePrefersJPEGAccept(t *testing.T) {
	var accept string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/Items/item-1/Images/Primary" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		accept = request.Header.Get("Accept")
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte{0xff, 0xd8, 0xff, 0xd9})
	}))
	defer server.Close()

	client := NewClient(server.URL, "emby-key", time.Second, "user-1")
	image, err := client.PrimaryImage(context.Background(), "item-1", 320)
	if err != nil {
		t.Fatal(err)
	}
	if image.ContentType != "image/jpeg" || len(image.Data) == 0 {
		t.Fatalf("image = %#v", image)
	}
	if !strings.HasPrefix(accept, "image/jpeg") {
		t.Fatalf("Accept = %q", accept)
	}
}
