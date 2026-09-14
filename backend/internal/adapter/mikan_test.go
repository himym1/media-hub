package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func (m *memoryOffline) EnsureFolder(_ context.Context, parentID, name string) (string, error) {
	return parentID + "/" + name, nil
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

	source := NewMikan(server.URL, "", time.Second, nil, nil)
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
	caseCandidate, ok := mikanCandidate(rssItem{
		Title: "[银色子弹字幕组&VCB-Studio] 名侦探柯南 独眼的残像 / Detective Conan M28: One-eyed Flashback 10-bit 1080p HEVC BDRip [MOVIE Fin]",
		Link:  "https://mikanani.me/Home/Episode/example",
	})
	if !ok || caseCandidate.MediaType != "movie" || caseCandidate.Season != 0 || caseCandidate.EpisodeStart != 0 {
		t.Fatalf("uppercase movie candidate = %+v, ok = %v", caseCandidate, ok)
	}
}

func TestMikanSearchUsesConfiguredProxy(t *testing.T) {
	var requestedURL string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedURL = r.URL.String()
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel></channel></rss>`))
	}))
	defer proxy.Close()
	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}

	source := NewMikan("http://mikan.invalid", "", time.Second, nil, proxyURL)
	if _, err := source.Search(context.Background(), "proxy test"); err != nil {
		t.Fatal(err)
	}
	if requestedURL != "http://mikan.invalid/RSS/Search?searchstr=proxy+test" {
		t.Fatalf("proxy request URL = %q", requestedURL)
	}
}

func TestMikanTransferSubmitsOfflineURL(t *testing.T) {
	offline := &memoryOffline{}
	source := NewMikan("https://mikanani.me", "", time.Second, offline, nil)
	reference, _ := json.Marshal(mikanReference{Title: "Van Helsing", URL: "magnet:?xt=urn:btih:abc"})
	result, err := source.StartTransfer(context.Background(), search.TransferRequest{
		Title: "范海辛", Reference: string(reference), DestinationID: "folder-1", IdempotencyKey: "job-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "pending" || result.FileID != "folder-1/范海辛" || result.Path != "范海辛" || offline.destination != "folder-1/范海辛" {
		t.Fatalf("result=%+v offline=%+v", result, offline)
	}
	if len(offline.urls) != 1 || !strings.HasPrefix(offline.urls[0], "magnet:") {
		t.Fatalf("urls = %#v", offline.urls)
	}
}

type uncertainOfflineError struct{}

func (uncertainOfflineError) Error() string             { return "submission uncertain" }
func (uncertainOfflineError) SubmissionUncertain() bool { return true }

func TestMikanTransferPreservesUncertainSubmission(t *testing.T) {
	offline := &memoryOffline{err: uncertainOfflineError{}}
	source := NewMikan("https://mikanani.me", "", time.Second, offline, nil)
	reference, _ := json.Marshal(mikanReference{Title: "Van Helsing", URL: "magnet:?xt=urn:btih:abc"})
	_, err := source.StartTransfer(context.Background(), search.TransferRequest{
		Reference: string(reference), DestinationID: "folder-1", IdempotencyKey: "job-1",
	})
	var failure search.Failure
	if !errors.As(err, &failure) || failure.Code != "source_submission_unknown" || !failure.Retryable {
		t.Fatalf("failure = %#v", err)
	}
}
