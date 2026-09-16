package emby

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNeedsTMDBIdentify(t *testing.T) {
	dump := Item{ID: "1", Name: "【高清影视之家发布 www.SSDSSE.com】长安的荔枝[60帧率版本]", ProviderIDs: map[string]string{}}
	if !NeedsTMDBIdentify(dump, "1356587") {
		t.Fatal("dump name should need identify")
	}
	matched := Item{ID: "1", Name: "长安的荔枝", ProviderIDs: map[string]string{"Tmdb": "1356587"}}
	if NeedsTMDBIdentify(matched, "1356587") {
		t.Fatal("clean identified title should not need identify")
	}
	wrongID := Item{ID: "1", Name: "长安的荔枝", ProviderIDs: map[string]string{"Tmdb": "1"}}
	if !NeedsTMDBIdentify(wrongID, "1356587") {
		t.Fatal("wrong tmdb id should need identify")
	}
}

func TestApplyTMDBMetadataFallsBackWhenRemoteSearchFails(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		paths = append(paths, request.Method+" "+request.URL.Path)
		switch {
		case request.URL.Path == "/Items/RemoteSearch/Movie":
			w.WriteHeader(http.StatusInternalServerError)
		case request.Method == http.MethodGet && request.URL.Path == "/Items/item-9":
			_, _ = w.Write([]byte(`{"Id":"item-9","Name":"dump","Type":"Movie","ProviderIds":{}}`))
		case request.Method == http.MethodPost && request.URL.Path == "/Items/item-9":
			body, _ := io.ReadAll(request.Body)
			if !strings.Contains(string(body), `"Tmdb":"333339"`) || !strings.Contains(string(body), `"Name":"头号玩家"`) {
				t.Fatalf("update body=%s", body)
			}
			w.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodPost && request.URL.Path == "/Items/item-9/Refresh":
			if request.URL.Query().Get("ReplaceAllMetadata") != "true" || request.URL.Query().Get("ReplaceAllImages") != "true" {
				t.Fatalf("refresh query=%s", request.URL.RawQuery)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second, "")
	if err := client.ApplyTMDBMetadata(context.Background(), "item-9", "movie", "头号玩家", 2018, "333339", true); err != nil {
		t.Fatalf("apply metadata: %v", err)
	}
	if len(paths) < 3 {
		t.Fatalf("paths=%q", paths)
	}
}
