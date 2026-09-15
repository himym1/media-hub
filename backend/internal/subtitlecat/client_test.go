package subtitlecat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExtractStudioCodePrefersHyphenatedPrefix(t *testing.T) {
	if got := ExtractStudioCode("Movie ABCD-123 1080p HEVC"); got != "ABCD-123" {
		t.Fatalf("code = %q", got)
	}
	if got := ExtractStudioCode("WEB-DL 1080p"); got != "" {
		t.Fatalf("denied code = %q", got)
	}
}

func TestSearchQueryFallsBackToTitle(t *testing.T) {
	if got := SearchQuery("Local Title", "", "/media/links/Local Title.mkv"); got != "Local Title" {
		t.Fatalf("query = %q", got)
	}
}

func TestSearchAndDownloadParseLanguageFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/index.php":
			_, _ = w.Write([]byte(`<a href="subs/1/ABCD-123.html">ABCD-123</a>`))
		case request.URL.Path == "/subs/1/ABCD-123.html":
			_, _ = w.Write([]byte(`<a href="/subs/2/ABCD-123-en.srt">en</a><a href="/subs/3/ABCD-123-zh-CN.srt">zh</a>`))
		case strings.HasSuffix(request.URL.Path, ".srt"):
			_, _ = w.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nhello\n"))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	client := NewClient(time.Second, nil)
	client.baseURL = server.URL
	client.lastCall = time.Now().Add(-time.Second)

	hits, err := client.Search(context.Background(), "ABCD-123")
	if err != nil || len(hits) != 2 || hits[0].Language != "zh-CN" || hits[1].Language != "en" {
		t.Fatalf("hits=%#v err=%v", hits, err)
	}
	name, body, err := client.Download(context.Background(), hits[0].ID)
	if err != nil || !strings.Contains(name, "zh-CN") || !strings.Contains(string(body), "hello") {
		t.Fatalf("download name=%q body=%q err=%v", name, body, err)
	}
}
