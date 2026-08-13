package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"media-hub/backend/internal/search"
)

type memoryOffline struct {
	destination string
	urls        []string
	err         error
}

func (m *memoryOffline) AddOfflineURLs(_ context.Context, destinationID string, urls []string) error {
	m.destination = destinationID
	m.urls = append([]string(nil), urls...)
	return m.err
}

func TestMikanSearchParsesRSSItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/RSS/Search" || r.URL.Query().Get("searchstr") != "范海辛" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0"><channel>
<item>
  <title>[ANi] Van Helsing - 05 [1080P][WEB-DL][AAC HEVC]</title>
  <enclosure url="https://mikanani.me/Download/20240101/abc.torrent" length="1234567890" type="application/x-bittorrent"/>
</item>
<item>
  <title>Van Helsing 剧场版 [2160p][AVC]</title>
  <enclosure url="magnet:?xt=urn:btih:abc" length="222" type="application/x-bittorrent"/>
</item>
</channel></rss>`))
	}))
	defer server.Close()

	source := NewMikan(server.URL, "", time.Second, nil)
	results, err := source.Search(context.Background(), "范海辛")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d", len(results))
	}
	if results[0].MediaType != "series" || results[0].EpisodeStart != 5 || results[0].Release.VideoCodec != "HEVC" {
		t.Fatalf("series candidate = %+v", results[0])
	}
	if results[1].MediaType != "movie" || results[1].Release.Resolution != "2160p" {
		t.Fatalf("movie candidate = %+v", results[1])
	}
}

func TestMikanTransferSubmitsOfflineURL(t *testing.T) {
	offline := &memoryOffline{}
	source := NewMikan("https://mikanani.me", "", time.Second, offline)
	reference, _ := json.Marshal(mikanReference{Title: "Van Helsing", URL: "magnet:?xt=urn:btih:abc"})
	result, err := source.StartTransfer(context.Background(), search.TransferRequest{
		Reference: string(reference), DestinationID: "folder-1", IdempotencyKey: "job-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" || result.FileID != "folder-1" || offline.destination != "folder-1" {
		t.Fatalf("result=%+v offline=%+v", result, offline)
	}
	if len(offline.urls) != 1 || !strings.HasPrefix(offline.urls[0], "magnet:") {
		t.Fatalf("urls = %#v", offline.urls)
	}
}
