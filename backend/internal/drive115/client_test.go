package drive115

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStatusReadsNonIdentifyingAccountMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"rt_space_info":{"all_use":{"size":12},"all_total":{"size":100}},"vip_info":{"level_name":"VIP","expire":1893456000}}}`))
	}))
	defer server.Close()

	client := NewClient("test-token", time.Second)
	client.endpoint = server.URL
	status, err := client.Status(context.Background())
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !status.Authorized || status.UsedBytes != 12 || status.TotalBytes != 100 || status.MemberLevel != "VIP" {
		t.Fatalf("unexpected status: %#v", status)
	}
}
