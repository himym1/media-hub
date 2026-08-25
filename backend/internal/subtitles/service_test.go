package subtitles

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"media-hub/backend/internal/assrt"
	"media-hub/backend/internal/emby"
)

type embyStub struct {
	target      emby.SubtitleTarget
	embyHits    []emby.RemoteSubtitle
	embyCalls   int
	downloaded  string
	refreshed   string
	searchErr   error
	downloadErr error
}

func (s *embyStub) SubtitleTarget(context.Context, string) (emby.SubtitleTarget, error) {
	return s.target, nil
}
func (s *embyStub) SearchRemoteSubtitles(context.Context, string, string) ([]emby.RemoteSubtitle, error) {
	s.embyCalls++
	return s.embyHits, s.searchErr
}
func (s *embyStub) DownloadRemoteSubtitle(_ context.Context, _, subtitleID string) error {
	s.downloaded = subtitleID
	return s.downloadErr
}
func (s *embyStub) RefreshItem(_ context.Context, itemID string) error {
	s.refreshed = itemID
	return nil
}

func TestSearchUsesAssrtForChineseAndSkipsEmptyEmby(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/sub/search" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"status":0,"sub":{"subs":[{"id":7,"native_name":"信号","subtype":"Srt","lang":{"desc":"简体","langlist":{"langchs":true}}}]}}`))
	}))
	defer server.Close()
	stub := &embyStub{target: emby.SubtitleTarget{ID: "ep-1", Type: "Episode", Name: "第一集", SeriesName: "信号", Season: 1, Episode: 1, Path: "/media3/115-strm/电视剧/信号/S01E01.strm"}}
	service := New(stub, assrt.NewClient(server.URL, "token", time.Second), func() string { return t.TempDir() })
	hits, err := service.Search(context.Background(), "ep-1", "chi")
	if err != nil || len(hits) != 1 || hits[0].ID != "assrt:7" || hits[0].ProviderName != "Assrt" {
		t.Fatalf("hits=%#v err=%v", hits, err)
	}
	if stub.embyCalls != 0 {
		t.Fatalf("emby was consulted %d times", stub.embyCalls)
	}
}

func TestSearchSkipsEmbyWhenAssrtReturnsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":0,"sub":{"subs":[]}}`))
	}))
	defer server.Close()
	stub := &embyStub{
		target:   emby.SubtitleTarget{ID: "ep-1", Type: "Episode", SeriesName: "信号", Season: 1, Episode: 1, Path: "/media/S01E01.strm"},
		embyHits: []emby.RemoteSubtitle{{ID: "opensubtitles-1", Name: "eng.srt"}},
	}
	service := New(stub, assrt.NewClient(server.URL, "token", time.Second), nil)
	hits, err := service.Search(context.Background(), "ep-1", "zh")
	if err != nil || len(hits) != 0 || stub.embyCalls != 0 {
		t.Fatalf("hits=%#v err=%v embyCalls=%d", hits, err, stub.embyCalls)
	}
}

func TestSearchFallsBackToEmbyWhenAssrtErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	stub := &embyStub{
		target:   emby.SubtitleTarget{ID: "ep-1", Type: "Episode", SeriesName: "信号", Season: 1, Episode: 1},
		embyHits: []emby.RemoteSubtitle{{ID: "opensubtitles-1", Language: "eng"}},
	}
	service := New(stub, assrt.NewClient(server.URL, "token", time.Second), nil)
	hits, err := service.Search(context.Background(), "ep-1", "chi")
	if err != nil || len(hits) != 1 || hits[0].ID != "opensubtitles-1" || stub.embyCalls != 1 {
		t.Fatalf("hits=%#v err=%v embyCalls=%d", hits, err, stub.embyCalls)
	}
}

func TestSearchEnglishUsesEmby(t *testing.T) {
	stub := &embyStub{embyHits: []emby.RemoteSubtitle{{ID: "eng-1"}}}
	service := New(stub, assrt.NewClient("https://api.assrt.net", "token", time.Second), nil)
	hits, err := service.Search(context.Background(), "item-1", "eng")
	if err != nil || len(hits) != 1 || stub.embyCalls != 1 {
		t.Fatalf("hits=%#v err=%v", hits, err)
	}
}

func TestDownloadWritesAssrtSidecarAndRefreshes(t *testing.T) {
	mount := t.TempDir()
	mediaDir := filepath.Join(mount, "电视剧", "信号")
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	media := filepath.Join(mediaDir, "S01E01.strm")
	if err := os.WriteFile(media, []byte("https://example/115/url/x"), 0o664); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/sub/detail":
			_, _ = w.Write([]byte(`{"status":0,"sub":{"subs":[{"id":7,"native_name":"信号","filelist":[{"f":"signal.chi.ass","url":"http://` + request.Host + `/signal.ass"}]}]}}`))
		case "/signal.ass":
			_, _ = w.Write([]byte("[Script Info]\nDialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,你好"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	stub := &embyStub{target: emby.SubtitleTarget{ID: "ep-1", Path: "/media3/115-strm/电视剧/信号/S01E01.strm"}}
	service := New(stub, assrt.NewClient(server.URL, "token", time.Second), func() string { return mount })
	if err := service.Download(context.Background(), "ep-1", "assrt:7"); err != nil {
		t.Fatal(err)
	}
	if stub.refreshed != "ep-1" {
		t.Fatalf("refreshed = %q", stub.refreshed)
	}
	if _, err := os.Stat(filepath.Join(mediaDir, "S01E01.chi.ass")); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadRoutesEmbyIDs(t *testing.T) {
	stub := &embyStub{}
	service := New(stub, nil, nil)
	if err := service.Download(context.Background(), "item-1", "opensubtitles-1"); err != nil {
		t.Fatal(err)
	}
	if stub.downloaded != "opensubtitles-1" {
		t.Fatalf("downloaded = %q", stub.downloaded)
	}
}
