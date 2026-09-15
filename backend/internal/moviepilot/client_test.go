package moviepilot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"media-hub/backend/internal/search"
)

func TestSearchMapsTorrentResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/mcp" || request.Header.Get("X-API-KEY") != "token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var envelope struct {
			Method string `json:"method"`
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		body, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(body, &envelope)
		switch {
		case envelope.Method == "tools/list":
			writeMCP(w, `{"tools":[]}`)
		case envelope.Params.Name == "search_media":
			writeMCPText(w, `[{"tmdb_id":603,"title":"The Matrix","year":1999,"media_type":"movie"}]`)
		case envelope.Params.Name == "search_torrents":
			writeMCPText(w, `{"total_count":1}`)
		case envelope.Params.Name == "get_search_results":
			writeMCPText(w, `{"total_count":1,"results":[{"torrent_info":{"title":"The Matrix 1999 2160p HEVC HDR 33.63 GB","size":"33.63 GB","seeders":12,"site_name":"织梦","torrent_url":"9d7e672:1"},"media_info":{"tmdb_id":603,"year":1999,"media_type":"movie"}}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", time.Second)
	results, err := client.Search(context.Background(), "The Matrix 1999")
	if err != nil || len(results) != 1 {
		t.Fatalf("results=%#v err=%v", results, err)
	}
	got := results[0]
	if got.TMDBID != "603" || got.Provider != "织梦" || got.SourceRef != "9d7e672:1" || got.TransferState != "downloadable" {
		t.Fatalf("candidate = %#v", got)
	}
	if got.Release.Resolution != "2160p" || got.Release.VideoCodec != "HEVC" || got.Release.SizeBytes != 33630000000 {
		t.Fatalf("release = %#v", got.Release)
	}
}

func TestStartDownloadSubmitsTorrentRef(t *testing.T) {
	var submitted []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		var envelope struct {
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		body, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(body, &envelope)
		if envelope.Params.Name == "add_download_tasks" {
			raw, _ := json.Marshal(envelope.Params.Arguments["torrent_url"])
			_ = json.Unmarshal(raw, &submitted)
			writeMCPText(w, `{"success":true}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", time.Second)
	if err := client.StartDownload(context.Background(), search.DownloadRequest{Reference: "9d7e672:1"}); err != nil {
		t.Fatal(err)
	}
	if len(submitted) != 1 || submitted[0] != "9d7e672:1" {
		t.Fatalf("submitted = %#v", submitted)
	}
}

func TestParseSizeBytes(t *testing.T) {
	if got := parseSizeBytes("33.63 GB"); got != 33630000000 {
		t.Fatalf("size = %d", got)
	}
}

func writeMCPText(w http.ResponseWriter, text string) {
	writeMCP(w, `{"content":[{"type":"text","text":`+jsonString(text)+`}]}`)
}

func writeMCP(w http.ResponseWriter, result string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":` + result + `}`))
}

func jsonString(value string) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
