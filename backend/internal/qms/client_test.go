package qms

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReadStatusNormalizesQMediaSyncRecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("api_key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/api/version":
			_, _ = w.Write([]byte(`{"version":"v1.2.3","date":"2026-06-01"}`))
		case "/api/sync/records":
			_, _ = w.Write([]byte(`{"code":200,"data":{"total":1,"records":[{"id":7,"status":2,"total":8,"new_strm":3,"new_meta":1,"new_upload":0,"base_cid":"file-7","created_at":1710000000,"finish_at":1710000060}]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second)
	status, err := client.ReadStatus(context.Background())
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if status.Version != "v1.2.3" || status.TotalSyncs != 1 {
		t.Fatalf("unexpected status: %#v", status)
	}
	if len(status.RecentSyncs) != 1 || status.RecentSyncs[0].State != "completed" {
		t.Fatalf("unexpected records: %#v", status.RecentSyncs)
	}
}

func TestSubmitManualSyncUsesVerifiedQMediaSyncContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/sync/manual" || request.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if request.URL.Query().Get("api_key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var payload ManualSyncRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload.PathID != "file-7" || payload.AccountID != 3 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"code":200,"message":"queued"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second)
	err := client.SubmitManualSync(context.Background(), ManualSyncRequest{
		PathID: "file-7", Path: "/Movies/Movie", TargetPath: "/strm/movies", AccountID: 3,
	})
	if err != nil {
		t.Fatalf("submit manual sync: %v", err)
	}
}
