package assrt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSearchNormalizesChineseHits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer assrt-token" || request.URL.Query().Get("token") != "assrt-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if request.URL.Path != "/v1/sub/search" || request.URL.Query().Get("q") != "信号 S01E01" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"status":0,"sub":{"subs":[{"id":42,"native_name":"信号 第一集","videoname":"Signal.S01E01","subtype":"Ass","release_site":"SubHD","lang":{"desc":"简体","langlist":{"langchs":true}}}]}}`))
	}))
	defer server.Close()

	hits, err := NewClient(server.URL, "assrt-token", time.Second).Search(context.Background(), "信号 S01E01", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != 42 || hits[0].Language != "chi" || hits[0].Format != "ass" {
		t.Fatalf("hits = %#v", hits)
	}
}

func TestDownloadFilePrefersAssFromFileList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/sub/detail":
			if request.URL.Query().Get("id") != "42" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"status":0,"sub":{"subs":[{"id":42,"native_name":"信号","url":"http://` + request.Host + `/archive.zip","filelist":[{"f":"signal.chi.ass","url":"http://` + request.Host + `/signal.ass"},{"f":"signal.rar","url":"http://` + request.Host + `/signal.rar"}]}]}}`))
		case "/signal.ass":
			_, _ = w.Write([]byte("[Script Info]\nDialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,你好"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	name, body, err := NewClient(server.URL, "assrt-token", time.Second).DownloadFile(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if name != "signal.chi.ass" || !looksLikeSubtitle(body) {
		t.Fatalf("name=%q body=%q", name, body)
	}
}

func TestSearchRejectsUnauthorizedToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":1}`))
	}))
	defer server.Close()
	_, err := NewClient(server.URL, "bad-token", time.Second).Search(context.Background(), "信号 S01E01", false)
	if err != ErrUnauthorized {
		t.Fatalf("err = %v", err)
	}
}

func TestCheckReportsUnconfigured(t *testing.T) {
	health := NewClient("", "", time.Second).Check(context.Background())
	if health.Status != "unconfigured" || health.ID != "assrt" {
		t.Fatalf("health = %#v", health)
	}
}
