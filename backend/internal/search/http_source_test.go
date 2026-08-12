package search

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPSourceSendsNormalizedQueryAndReadsCandidate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/search" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if request.URL.Query().Get("query") != "范海辛" || request.URL.Query().Get("limit") != "50" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"release-1","title":"范海辛","year":2004,"mediaType":"movie","posterUrl":"https://images.local/poster.jpg","release":{"resolution":"2160p","videoCodec":"HEVC","sizeBytes":100},"reference":"opaque-reference"}]}`))
	}))
	defer server.Close()

	source := NewHTTPSource("frame", "帧影", server.URL, "test-token", time.Second)
	results, err := source.Search(context.Background(), "范海辛")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].SourceRef != "opaque-reference" {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestHTTPSourceTransferUsesIdempotencyAndPollsOperation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/transfer":
			if request.Method != http.MethodPost || request.Header.Get("Idempotency-Key") != "job_transfer" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"operationId":"operation_1","status":"pending"}`))
		case "/api/transfer/operation_1":
			_, _ = w.Write([]byte(`{"operationId":"operation_1","status":"completed","fileId":"100","path":"/Movies/Movie","isFile":false}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	source := NewHTTPSource("frame", "帧影", server.URL+"/api", "test-token", time.Second)
	started, err := source.StartTransfer(context.Background(), TransferRequest{
		Reference: "private-reference", DestinationID: "200", IdempotencyKey: "job_transfer",
	})
	if err != nil || started.Status != "pending" {
		t.Fatalf("start transfer: result=%#v err=%v", started, err)
	}
	completed, err := source.TransferStatus(context.Background(), 1, started.OperationID)
	if err != nil || completed.Status != "completed" || completed.FileID != "100" {
		t.Fatalf("poll transfer: result=%#v err=%v", completed, err)
	}
}
