package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"media-hub/backend/internal/search"
)

type juyingTransferTarget struct {
	shareCode       string
	receiveCode     string
	magnets         []string
	shareVideoNames []string
	inspectErr      error
	inspectCalls    int
}

func (t *juyingTransferTarget) ReceiveShare(_ context.Context, _ string, shareCode, receiveCode string, _ []string) error {
	t.shareCode = shareCode
	t.receiveCode = receiveCode
	return nil
}

func (t *juyingTransferTarget) ShareVideoNames(_ context.Context, _, _ string) ([]string, error) {
	t.inspectCalls++
	if t.inspectErr != nil {
		return nil, t.inspectErr
	}
	if t.shareVideoNames == nil {
		return []string{"Van.Helsing.2004.2160p.mkv"}, nil
	}
	return append([]string(nil), t.shareVideoNames...), nil
}

func (t *juyingTransferTarget) AddOfflineURLs(_ context.Context, _ string, urls []string) error {
	t.magnets = append([]string(nil), urls...)
	return nil
}

func TestJuyingSearchAndTransfersSupportedResources(t *testing.T) {
	const magnet = "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=Van.Helsing.2004.1080p"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-App-ID") != "app-id" || r.Header.Get("X-App-Key") != "app-key" {
			t.Errorf("credentials headers were not set")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/dev/movies/":
			if r.URL.Query().Get("q") != "范海辛" || r.URL.Query().Get("page_size") != "50" {
				t.Errorf("query = %v", r.URL.Query())
			}
			_, _ = w.Write([]byte(`{"status":"success","results":[{"id":1,"title":"范海辛 Van Helsing","release_year":"2004","movie_type":"movie","tmdb_id":7131}]}`))
		case "/api/dev/movie/1/resources/":
			_, _ = w.Write([]byte(`{"status":"success","title":"范海辛","resources":[` +
				`{"id":11,"resource_type":"115","share_link":"https://115.com/s/shareABC123","extraction_code":"WENG","description":"Van.Helsing.2004.2160p.HEVC","file_size":"12.5 GB"},` +
				`{"id":12,"resource_type":"magnet","share_link":"` + magnet + `","description":"Van.Helsing.2004.1080p"},` +
				`{"id":13,"resource_type":"ed2k","share_link":"ed2k://bad","description":"ignored"}` +
				`]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	target := &juyingTransferTarget{}
	source := NewJuying(server.URL, "app-id", "app-key", time.Second, target, target, nil)
	source.client = server.Client()
	results, err := source.Search(context.Background(), "范海辛")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %#v", results)
	}
	if results[0].Title != "范海辛 Van Helsing" || results[0].Year != 2004 || results[0].MediaType != "movie" ||
		results[0].TMDBID != "7131" || results[0].ReleaseTitle != "Van.Helsing.2004.2160p.HEVC" ||
		results[0].Release.Resolution != "2160p" || results[0].Release.SizeBytes != 13421772800 {
		t.Fatalf("share candidate = %+v", results[0])
	}
	if strings.Contains(results[0].SourceRef, "115.com") || strings.Contains(results[0].SourceRef, "app-key") {
		t.Fatalf("reference contains upstream URL or credential: %s", results[0].SourceRef)
	}
	if _, err := source.StartTransfer(context.Background(), search.TransferRequest{Reference: results[0].SourceRef, DestinationID: "dest", IdempotencyKey: "share-op"}); err != nil {
		t.Fatal(err)
	}
	if target.shareCode != "shareABC123" || target.receiveCode != "WENG" {
		t.Fatalf("share = %q/%q", target.shareCode, target.receiveCode)
	}
	if results[1].SourceRef != "" || results[1].TransferState != "unavailable" {
		t.Fatalf("magnet candidate remained transferable: %+v", results[1])
	}
	if len(target.magnets) != 0 {
		t.Fatalf("magnets = %#v", target.magnets)
	}
}

func TestJuyingRejectsUnverifiableMagnetBeforeOffline(t *testing.T) {
	target := &juyingTransferTarget{}
	source := NewJuying("https://www.jying.top", "app-id", "app-key", time.Second, target, target, nil)
	reference, err := json.Marshal(juyingReference{
		Kind: "magnet", Title: "Van.Helsing.2004.2160p",
		Magnet: "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=Van.Helsing.2004.2160p",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = source.StartTransfer(context.Background(), search.TransferRequest{
		Reference: string(reference), DestinationID: "dest", IdempotencyKey: "op",
	})
	var failure search.Failure
	if !errors.As(err, &failure) || failure.Code != "source_identity_mismatch" || failure.Retryable {
		t.Fatalf("failure = %#v", err)
	}
	if len(target.magnets) != 0 {
		t.Fatalf("offline target was called: %+v", target)
	}
}

func TestJuyingRejectsShareIdentityMismatchBeforeReceive(t *testing.T) {
	target := &juyingTransferTarget{shareVideoNames: []string{"Wrong.Movie.2020.2160p.mkv"}}
	source := NewJuying("https://www.jying.top", "app-id", "app-key", time.Second, target, target, nil)
	reference, err := json.Marshal(juyingReference{Kind: "share", Title: "Van.Helsing.2004.2160p", ShareCode: "shareABC123", ReceiveCode: "WENG"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = source.StartTransfer(context.Background(), search.TransferRequest{Reference: string(reference), DestinationID: "dest", IdempotencyKey: "op"})
	var failure search.Failure
	if !errors.As(err, &failure) || failure.Code != "source_identity_mismatch" || failure.Retryable {
		t.Fatalf("failure = %#v", err)
	}
	if target.inspectCalls != 1 || target.shareCode != "" {
		t.Fatalf("target = %+v", target)
	}
}

func TestJuyingCandidateRejectsUnsupportedOrMalformedLinks(t *testing.T) {
	resources := []juyingResource{
		{ResourceType: "115", ShareLink: "https://evil.example/s/code", ExtractionCode: "ABCD"},
		{ResourceType: "magnet", ShareLink: "magnet:?xt=urn:btih:bad"},
		{ResourceType: "ed2k", ShareLink: "ed2k://example"},
	}
	for _, resource := range resources {
		if candidate, ok := juyingCandidate("1", "title", 2020, "movie", resource); ok {
			t.Fatalf("candidate = %+v", candidate)
		}
	}
}

func TestJuyingSeriesWithoutExplicitEpisodeRangeRemainsUntransferable(t *testing.T) {
	resource := juyingResource{ID: json.RawMessage(`1`), ResourceType: "115", ShareLink: "https://115.com/s/shareABC123?password=WENG", Description: "Series Complete"}
	candidate, ok := juyingCandidate("1", "Series", 2020, "series", resource)
	if !ok {
		t.Fatal("candidate was rejected")
	}
	if candidate.Season != 0 || candidate.EpisodeStart != 0 || candidate.EpisodeEnd != 0 {
		t.Fatalf("candidate = %+v", candidate)
	}
}

func TestJuyingRejectsFailedAPIStatus(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"error","message":"invalid app"}`))
	}))
	defer server.Close()
	source := NewJuying(server.URL, "app-id", "app-key", time.Second, nil, nil, nil)
	source.client = server.Client()
	if _, err := source.Search(context.Background(), "test"); err == nil {
		t.Fatal("failed API status was accepted")
	}
}
func TestJuyingRejectsForgedShareReferenceBeforeReceiver(t *testing.T) {
	target := &juyingTransferTarget{}
	source := NewJuying("https://www.jying.top", "app-id", "app-key", time.Second, target, target, nil)
	_, err := source.StartTransfer(context.Background(), search.TransferRequest{
		Reference:     `{"kind":"share","title":"Movie","shareCode":"bad!","receiveCode":"ABCD"}`,
		DestinationID: "dest", IdempotencyKey: "op",
	})
	if err == nil {
		t.Fatal("forged share reference was accepted")
	}
	if target.shareCode != "" || len(target.magnets) != 0 {
		t.Fatalf("target was called: %+v", target)
	}
}
