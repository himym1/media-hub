package drive115

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"media-hub/backend/internal/playback"
)

func TestAuthServiceResolvesPlaybackByTrustedFileMetadata(t *testing.T) {
	const playbackUA = "MediaHub-Test-Player"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Cookie") != "UID=uid; CID=cid; SEID=seid" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/info":
			if request.URL.Query().Get("file_id") != "20" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"state":true,"data":[{"file_id":"20","category_id":"10","file_name":"Movie.mkv","pick_code":"pick-secret"}]}`))
		case "/download":
			if request.URL.Query().Get("pickcode") != "pick-secret" || request.Header.Get("User-Agent") != playbackUA {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"state":true,"url":{"url":"https://cdn.example/Movie.mkv?token=short"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient("UID=uid; CID=cid; SEID=seid", time.Second)
	client.fileInfoURL = server.URL + "/info"
	client.downloadURL = server.URL + "/download"
	service := NewAuthService(nil, nil, client, time.Second)
	value, err := service.Resolve(context.Background(), "10", "20", playbackUA)
	if err != nil {
		t.Fatal(err)
	}
	if value.Name != "Movie.mkv" || value.URL != "https://cdn.example/Movie.mkv?token=short" {
		t.Fatalf("media = %#v", value)
	}
}

func TestResolvePlaybackRejectsMismatchedParentAndUnknownFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("file_id") == "20" {
			_, _ = w.Write([]byte(`{"state":true,"data":[{"fid":"20","cid":"11","n":"Movie.mkv","pc":"pick-secret"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"state":true,"data":[]}`))
	}))
	defer server.Close()
	client := NewClient("UID=uid", time.Second)
	client.fileInfoURL = server.URL
	service := NewAuthService(nil, nil, client, time.Second)
	for _, fileID := range []string{"20", "99"} {
		_, err := service.Resolve(context.Background(), "10", fileID, "player")
		if !errors.Is(err, playback.ErrNotFound) {
			t.Fatalf("fileID=%s error=%v", fileID, err)
		}
	}
}

func TestResolvePlaybackRequiresPreparedSession(t *testing.T) {
	service := NewAuthService(nil, nil, NewClient("", time.Second), time.Second)
	_, err := service.Resolve(context.Background(), "10", "20", "player")
	if !errors.Is(err, playback.ErrSourceNotConfigured) {
		t.Fatalf("error=%v", err)
	}
}
