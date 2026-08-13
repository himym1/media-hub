package adapter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"media-hub/backend/internal/search"
)

const sidhubTestHash = "0123456789abcdef0123456789abcdef01234567"

func TestSidhubSearchParsesAnonymousMovieResources(t *testing.T) {
	server := newSidhubTestServer(t)
	defer server.Close()

	source := NewSidhub(server.URL, time.Second, nil, nil)
	source.client = server.Client()

	results, err := source.Search(context.Background(), "范海辛")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d", len(results))
	}
	movie := results[0]
	if movie.Title != "范海辛" || movie.Year != 2004 || movie.MediaType != "" {
		t.Fatalf("movie = %+v", movie)
	}
	if movie.Release.Resolution != "2160p" || movie.Release.VideoCodec != "HEVC" || movie.Release.DynamicRange != "HDR10" {
		t.Fatalf("movie release = %+v", movie.Release)
	}
	if movie.Release.SizeBytes != 13421772800 {
		t.Fatalf("size = %d", movie.Release.SizeBytes)
	}
	var reference sidhubReference
	if json.Unmarshal([]byte(movie.SourceRef), &reference) != nil || reference.LinkPath != "/link_start/?seed_id=101" || strings.Contains(reference.LinkPath, "movie_title") {
		t.Fatalf("reference = %+v", reference)
	}
	series := results[1]
	if series.MediaType != "series" || series.Season != 2 || series.EpisodeStart != 3 || series.EpisodeEnd != 5 {
		t.Fatalf("series = %+v", series)
	}
}

func TestSidhubTransferDecodesAndSubmitsOnlyValidMagnet(t *testing.T) {
	server := newSidhubTestServer(t)
	defer server.Close()
	offline := &memoryOffline{}
	source := NewSidhub(server.URL, time.Second, offline, nil)
	source.client = server.Client()
	reference, _ := json.Marshal(sidhubReference{Title: "Van Helsing", LinkPath: "/link_start/?seed_id=101"})

	result, err := source.StartTransfer(context.Background(), search.TransferRequest{
		Reference: string(reference), DestinationID: "movie-folder", IdempotencyKey: "request-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" || offline.destination != "movie-folder" || len(offline.urls) != 1 {
		t.Fatalf("result=%+v offline=%+v", result, offline)
	}
	if offline.urls[0] != "magnet:?xt=urn:btih:"+sidhubTestHash {
		t.Fatalf("offline URL = %q", offline.urls[0])
	}
}

func TestSidhubRejectsInvalidReferencesAndDecodedResources(t *testing.T) {
	source := NewSidhub("https://sidhub.cc", time.Second, &memoryOffline{}, nil)
	for _, raw := range []string{
		`{"title":"Movie","linkPath":"https://evil.example/link_start/?seed_id=1"}`,
		`{"title":"Movie","linkPath":"/link_start/?seed_id=../1"}`,
		`{"title":"Movie","linkPath":"/movies/1/"}`,
	} {
		if _, err := source.StartTransfer(context.Background(), search.TransferRequest{Reference: raw}); err == nil {
			t.Fatalf("reference %s was accepted", raw)
		}
	}
	for _, resource := range []string{
		"https://evil.example/file.torrent",
		"magnet:?xt=urn:btih:short",
		"magnet:?xt=urn:sha1:" + sidhubTestHash,
		"magnet:?xt=urn:btih:" + sidhubTestHash + "\nhttps://evil.example",
		"magnet:opaque?xt=urn:btih:" + sidhubTestHash,
	} {
		encoded := base64.StdEncoding.EncodeToString([]byte(resource))
		if _, err := decodeSidhubMagnet([]byte(`<script>const data = "` + encoded + `";</script>`)); err == nil {
			t.Fatalf("resource %q was accepted", resource)
		}
	}
}

func TestSidhubUsesConfiguredProxy(t *testing.T) {
	proxyURL, err := url.Parse("http://proxy.local:8080")
	if err != nil {
		t.Fatal(err)
	}
	source := NewSidhub("https://sidhub.cc", time.Second, nil, proxyURL)
	request, err := http.NewRequest(http.MethodGet, "https://sidhub.cc/s/test/", nil)
	if err != nil {
		t.Fatal(err)
	}
	transport := source.client.Transport.(*http.Transport)
	resolved, err := transport.Proxy(request)
	if err != nil || resolved.String() != proxyURL.String() {
		t.Fatalf("proxy = %v, err = %v", resolved, err)
	}
}

func TestSidhubFetchesDetailsConcurrentlyAndPreservesOrder(t *testing.T) {
	entered := make(chan string, 2)
	release := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/s/test/":
			_, _ = w.Write([]byte(`<div class="cover"><a class="image" title="First" href="/movies/1/"></a><h2>First</h2></div><div class="cover"><a class="image" title="Second" href="/movies/2/"></a><h2>Second</h2></div>`))
		case "/movies/1/", "/movies/2/":
			entered <- r.URL.Path
			<-release
			seedID := strings.Trim(r.URL.Path, "/movies/")
			_, _ = w.Write([]byte(`<h1 id="cover">` + map[string]string{"1": "First", "2": "Second"}[seedID] + `</h1><ul class="seeds"><li><a title="release" href="/link_start/?seed_id=` + seedID + `">release</a></li></ul>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	source := NewSidhub(server.URL, time.Second, nil, nil)
	source.client = server.Client()
	type outcome struct {
		results []search.Candidate
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		results, err := source.Search(context.Background(), "test")
		done <- outcome{results: results, err: err}
	}()
	for range 2 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("detail requests did not run concurrently")
		}
	}
	close(release)
	result := <-done
	if result.err != nil {
		t.Fatal(result.err)
	}
	if len(result.results) != 2 || result.results[0].Title != "First" || result.results[1].Title != "Second" {
		t.Fatalf("results = %#v", result.results)
	}
}

func newSidhubTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/s/范海辛/":
			_, _ = w.Write([]byte(`<div class="cover-container">
<div class="cover"><a class="image" title="范海辛 Van Helsing" href="/movies/7131/"></a><ul><li><h2><a href="/movies/7131/">#</a> 范海辛</h2></li><li>2004 / 电影</li></ul></div>
</div>`))
		case r.URL.Path == "/movies/7131/":
			_, _ = w.Write([]byte(`<h1 id="cover"><a>#</a> 范海辛 Van Helsing</h1>
<ul class="seeds">
<li><a title="Van Helsing 2004 2160p BluRay x265 HDR10 [12.5G]" href="/link_start/?seed_id=101&amp;movie_title=ignored">release</a> / <code class="size">12.5G</code></li>
<li><a title="Van Helsing S02E03-E05 1080p x264 [3G]" href="/link_start/?seed_id=102">series</a> / <code class="size">3G</code></li>
<li><a title="external" href="https://evil.example/link_start/?seed_id=103">external</a></li>
<li><a title="` + strings.Repeat("x", 301) + `" href="/link_start/?seed_id=104">oversized</a></li>
</ul>`))
		case r.URL.Path == "/link_start/" && r.URL.Query().Get("seed_id") == "101":
			encoded := base64.StdEncoding.EncodeToString([]byte("magnet:?xt=urn:btih:" + sidhubTestHash))
			_, _ = w.Write([]byte(`<script>const data = "` + encoded + `";</script>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}
