package adapter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"media-hub/backend/internal/search"
)

func TestPansouSearchKeepsOnly115Shares(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/search" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer pansou-token" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"kw":"范海辛"`) || !strings.Contains(string(body), `"115"`) {
			t.Errorf("body = %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"total":3,"results":[` +
			`{"unique_id":"a","title":"范海辛 2160p","content":"约12.5 GB","links":[` +
			`{"type":"115","url":"https://115.com/s/shareABC123?password=WENG","password":"WENG","work_title":"Van.Helsing.2004.2160p.HEVC"},` +
			`{"type":"quark","url":"https://pan.quark.cn/s/ignored","password":"abcd"}]},` +
			`{"unique_id":"b","title":"重复分享","links":[{"type":"115","url":"https://115.com/s/shareABC123","password":"WENG"}]},` +
			`{"unique_id":"c","title":"磁力","links":[{"type":"magnet","url":"magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567"}]}` +
			`]}}`))
	}))
	defer server.Close()

	target := &juyingTransferTarget{}
	source := NewPansou(server.URL, "pansou-token", time.Second, target)
	results, err := source.Search(context.Background(), "范海辛")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %#v", results)
	}
	if results[0].Title != "范海辛" || results[0].Year != 2004 || results[0].ReleaseTitle != "Van.Helsing.2004.2160p.HEVC" ||
		results[0].Release.Resolution != "2160p" || results[0].TransferState != "available" {
		t.Fatalf("candidate = %+v", results[0])
	}
	if strings.Contains(results[0].SourceRef, "115.com") || strings.Contains(results[0].SourceRef, "pansou-token") || strings.Contains(results[0].SourceRef, "quark") {
		t.Fatalf("reference leaked upstream URL or credential: %s", results[0].SourceRef)
	}
	result, err := source.StartTransfer(context.Background(), search.TransferRequest{
		Title: "范海辛", MediaType: "movie", Reference: results[0].SourceRef, DestinationID: "dest", IdempotencyKey: "share-op",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" || target.shareCode != "shareABC123" || target.receiveCode != "WENG" || target.destinationID == "" {
		t.Fatalf("transfer = %+v target = %+v", result, target)
	}
}

func TestPansouSearchFallsBackToMerged115(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"merged_by_type":{"115":[{"url":"115.com/s/shareXYZ9","password":"ab12","note":"Van.Helsing.2004.1080p"}],"quark":[{"url":"https://pan.quark.cn/s/nope"}]}}}`))
	}))
	defer server.Close()

	source := NewPansou(server.URL, "", time.Second, nil)
	results, err := source.Search(context.Background(), "范海辛")
	if err != nil || len(results) != 1 {
		t.Fatalf("results = %#v err=%v", results, err)
	}
	var reference pansouReference
	if err := json.Unmarshal([]byte(results[0].SourceRef), &reference); err != nil || reference.ShareCode != "shareXYZ9" || reference.ReceiveCode != "ab12" {
		t.Fatalf("reference = %+v err=%v", reference, err)
	}
}

func TestPansouSearchRejectsUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := NewPansou(server.URL, "bad", time.Second, nil).Search(context.Background(), "范海辛")
	failure, ok := err.(search.Failure)
	if !ok || failure.Code != "source_unauthorized" || failure.Retryable {
		t.Fatalf("err = %#v", err)
	}
}

func TestParsePansou115ShareURLAcceptsLooseForms(t *testing.T) {
	code, receive, err := parsePansou115ShareURL("https://115.com/s/shareABC123?password=WENG&from=tg", "")
	if err != nil || code != "shareABC123" || receive != "WENG" {
		t.Fatalf("query url = %q %q %v", code, receive, err)
	}
	code, receive, err = parsePansou115ShareURL("115cdn.com/s/shareABC123", "xy9z")
	if err != nil || code != "shareABC123" || receive != "xy9z" {
		t.Fatalf("host-only = %q %q %v", code, receive, err)
	}
	if _, _, err := parsePansou115ShareURL("https://pan.quark.cn/s/nope", "abcd"); err == nil {
		t.Fatal("quark URL was accepted")
	}
}
