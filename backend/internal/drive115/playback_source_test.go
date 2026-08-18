package drive115

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"media-hub/backend/internal/playback"
)

func TestPlaybackFileInfoReadsTrustedMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Cookie") != "UID=uid; CID=cid; SEID=seid" || request.URL.Query().Get("file_id") != "20" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"state":true,"data":[{"file_id":"20","category_id":"10","file_name":"Movie.mkv","pick_code":"pick-secret"}]}`))
	}))
	defer server.Close()

	client := NewClient("UID=uid; CID=cid; SEID=seid", time.Second)
	client.fileInfoURL = server.URL
	info, err := client.playbackFileInfo(context.Background(), "20")
	if err != nil {
		t.Fatal(err)
	}
	if info.FileID != "20" || info.ParentID != "10" || info.Name != "Movie.mkv" || info.PickCode != "pick-secret" {
		t.Fatalf("info = %#v", info)
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
		_, err := service.ResolveDrive115(context.Background(), playback.Drive115Target{ParentID: "10", FileID: fileID}, "player")
		if !errors.Is(err, playback.ErrNotFound) {
			t.Fatalf("fileID=%s error=%v", fileID, err)
		}
	}
}

func TestResolvePlaybackRequiresPreparedSession(t *testing.T) {
	service := NewAuthService(nil, nil, NewClient("", time.Second), time.Second)
	_, err := service.ResolveDrive115(context.Background(), playback.Drive115Target{ParentID: "10", FileID: "20"}, "player")
	if !errors.Is(err, playback.ErrSourceNotConfigured) {
		t.Fatalf("error=%v", err)
	}
}

func TestResolvePickCodeUsesEncryptedDownurlRequest(t *testing.T) {
	const playbackUA = "MediaHub-Test-Player"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/download" || request.Method != http.MethodPost || request.URL.Query().Get("t") == "" || request.Header.Get("User-Agent") != playbackUA {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := request.ParseForm(); err != nil || request.PostForm.Get("data") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if _, err := base64.StdEncoding.DecodeString(request.PostForm.Get("data")); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"state":false,"error":"not available"}`))
	}))
	defer server.Close()
	client := NewClient("UID=uid; CID=cid; SEID=seid", time.Second)
	client.downloadURL = server.URL + "/download"
	service := NewAuthService(nil, nil, client, time.Second)
	_, err := service.ResolvePickCode(context.Background(), "abcd1234", "Movie.mkv", playbackUA)
	if !errors.Is(err, ErrUpstreamResponse) {
		t.Fatalf("error=%v", err)
	}
}

func TestM115DownloadURLExtractsNestedHTTPSURL(t *testing.T) {
	payload := []byte(`{"200":{"file_name":"Movie.mkv","url":{"url":"https://cdn.example/Movie.mkv?token=short"}}}`)
	if value := m115DownloadURL(payload); value != "https://cdn.example/Movie.mkv?token=short" {
		t.Fatalf("url=%q", value)
	}
}
