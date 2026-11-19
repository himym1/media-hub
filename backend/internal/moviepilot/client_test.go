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

func TestSearchPrefersMediaMatchingQueryYear(t *testing.T) {
	var torrentTMDB any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		var envelope struct {
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		body, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(body, &envelope)
		switch envelope.Params.Name {
		case "search_media":
			writeMCPText(w, `[{"tmdb_id":912649,"title":"毒液：最后一舞","year":2024,"media_type":"movie"},{"tmdb_id":335983,"title":"毒液：致命守护者","year":2018,"media_type":"movie"}]`)
		case "search_torrents":
			torrentTMDB = envelope.Params.Arguments["tmdb_id"]
			writeMCPText(w, `{"total_count":1}`)
		case "get_search_results":
			writeMCPText(w, `{"total_count":1,"results":[{"torrent_info":{"title":"Venom 2018 1080p","size":"8 GB","seeders":4,"site_name":"织梦","torrent_url":"venom:1"},"media_info":{"tmdb_id":335983,"year":2018,"media_type":"movie"}}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", time.Second)
	results, err := client.Search(context.Background(), "毒液 2018")
	if err != nil || len(results) != 1 {
		t.Fatalf("results=%#v err=%v", results, err)
	}
	if results[0].TMDBID != "335983" {
		t.Fatalf("candidate = %#v", results[0])
	}
	if torrentTMDB != float64(335983) {
		t.Fatalf("search_torrents tmdb_id = %#v", torrentTMDB)
	}
}

func TestStartDownloadWaitsLongerThanSearchClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		time.Sleep(150 * time.Millisecond)
		writeMCPText(w, `{"success":true}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", 50*time.Millisecond)
	if err := client.StartDownload(context.Background(), search.DownloadRequest{Reference: "venom:1"}); err != nil {
		t.Fatal(err)
	}
}

func TestPublicFailureMapsTimeout(t *testing.T) {
	err := publicFailure(context.DeadlineExceeded)
	failure, ok := err.(search.Failure)
	if !ok || failure.Message != "MoviePilot 提交下载超时" || !failure.Retryable {
		t.Fatalf("failure = %#v", err)
	}
}

func TestStartDownloadRejectsStaleSearchReference(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		writeMCPText(w, "任务添加失败：001dbd6:13 引用无效，请重新使用 get_search_results 查看搜索结果")
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", time.Second)
	err := client.StartDownload(context.Background(), search.DownloadRequest{Reference: "stale:1"})
	failure, ok := err.(search.Failure)
	if !ok || failure.Code != "source_stale" || failure.Retryable {
		t.Fatalf("err = %#v", err)
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

func TestV3CheckSearchAndDownload(t *testing.T) {
	var seenOperations []string
	var lastSearchTorrentsQuery map[string]any
	var lastDownloadBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		var envelope struct {
			Method string `json:"method"`
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		body, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(body, &envelope)

		if envelope.Method == "tools/list" {
			writeMCP(w, `{"tools":[{"name":"moviepilot_api"}]}`)
			return
		}
		if envelope.Params.Name != "moviepilot_api" {
			t.Fatalf("unexpected tool %q", envelope.Params.Name)
		}
		op, _ := envelope.Params.Arguments["operation_id"].(string)
		seenOperations = append(seenOperations, op)

		switch op {
		case "media.search":
			writeMCPText(w, `{"success":true,"data":[{"media_source":"themoviedb","type":"电视剧","title":"绝命毒师","year":"2008","tmdb_id":1396,"media_id":"1396"}]}`)
		case "search.torrents":
			lastSearchTorrentsQuery, _ = envelope.Params.Arguments["query"].(map[string]any)
			writeMCPText(w, `{"collection":{"result_count":1,"total_count":1},"success":true,"data":[{"torrent_info":{"site":2,"site_name":"下水道","site_cookie":"secret-cookie","enclosure":"https://sewerpt.com/download.php?id=42&passkey=secret","page_url":"https://sewerpt.com/details.php?id=42","title":"Breaking Bad S01 2008 1080p HEVC","size":8922794557.0,"seeders":88},"media_info":{"media_source":"themoviedb","media_id":"1396","tmdb_id":1396,"type":"电视剧","title":"绝命毒师","year":"2008"}}]}`)
		case "search.results":
			writeMCPText(w, `{"success":true,"data":{"results":[{"torrent_info":{"site":2,"site_name":"下水道","site_cookie":"secret-cookie","enclosure":"https://sewerpt.com/download.php?id=42&passkey=secret","page_url":"https://sewerpt.com/details.php?id=42","title":"Breaking Bad S01 2008 1080p HEVC","size":8922794557.0,"seeders":88},"media_info":{"media_source":"themoviedb","media_id":"1396","tmdb_id":1396,"type":"电视剧","title":"绝命毒师","year":"2008"}}]}}`)
		case "download.add":
			lastDownloadBody, _ = envelope.Params.Arguments["body"].(map[string]any)
			writeMCPText(w, `{"success":true,"data":{"download_id":"qb-1"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", time.Second)
	health := client.Check(context.Background())
	if health.Status != "healthy" {
		t.Fatalf("health = %#v", health)
	}

	results, err := client.Search(context.Background(), "绝命毒师")
	if err != nil || len(results) != 1 {
		t.Fatalf("results=%#v err=%v", results, err)
	}
	got := results[0]
	if got.MediaType != "series" || got.TMDBID != "1396" || got.Year != 2008 || got.Provider != "下水道" {
		t.Fatalf("candidate = %#v", got)
	}
	if got.Release.SizeBytes != 8922794557 || got.Release.Resolution != "1080p" || got.Release.VideoCodec != "HEVC" {
		t.Fatalf("release = %#v", got.Release)
	}
	if got.SourceRef == "" || got.SourceRef[:5] != "mpv3:" || got.SourceRef == "https://sewerpt.com/download.php?id=42&passkey=secret" {
		t.Fatalf("unsafe or invalid SourceRef = %q", got.SourceRef)
	}
	if lastSearchTorrentsQuery["mtype"] != "tv" || lastSearchTorrentsQuery["media_source"] != "themoviedb" {
		t.Fatalf("search.torrents query = %#v", lastSearchTorrentsQuery)
	}

	// 验证缓存命中直接调用 download.add
	if err := client.StartDownload(context.Background(), search.DownloadRequest{Reference: got.SourceRef}); err != nil {
		t.Fatalf("StartDownload cached err = %v", err)
	}
	if lastDownloadBody["media_id"] != "1396" || lastDownloadBody["media_source"] != "themoviedb" {
		t.Fatalf("download.add body = %#v", lastDownloadBody)
	}
	torrentIn, _ := lastDownloadBody["torrent_in"].(map[string]any)
	if torrentIn == nil || torrentIn["title"] != "Breaking Bad S01 2008 1080p HEVC" {
		t.Fatalf("torrent_in = %#v", torrentIn)
	}

	// 模拟新实例（冷缓存）时通过 search.results 回退并提交 download.add
	coldClient := NewClient(server.URL, "token", time.Second)
	if err := coldClient.StartDownload(context.Background(), search.DownloadRequest{Reference: got.SourceRef}); err != nil {
		t.Fatalf("StartDownload cold fallback err = %v", err)
	}
	if len(seenOperations) < 4 {
		t.Fatalf("seenOperations = %#v", seenOperations)
	}
}

func TestV3SearchTorrentsTruncatedFallsBackToSearchResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		var envelope struct {
			Method string `json:"method"`
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		body, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(body, &envelope)
		if envelope.Method == "tools/list" {
			writeMCP(w, `{"tools":[{"name":"moviepilot_api"}]}`)
			return
		}
		switch envelope.Params.Arguments["operation_id"] {
		case "media.search":
			writeMCPText(w, `{"success":true,"data":[{"media_source":"themoviedb","type":"电影","title":"黑客帝国","year":"1999","tmdb_id":603,"media_id":"603"}]}`)
		case "search.torrents":
			writeMCPText(w, `{"collection":{"result_count":0,"total_count":1},"success":true,"data":[]}`)
		case "search.results":
			writeMCPText(w, `{"success":true,"data":{"results":[{"torrent_info":{"site":2,"site_name":"下水道","enclosure":"https://sewerpt.com/download.php?id=1&passkey=k","title":"The Matrix 1999 2160p HEVC","size":30000000000,"seeders":10},"media_info":{"media_source":"themoviedb","media_id":"603","tmdb_id":603,"type":"电影","title":"黑客帝国","year":"1999"}}]}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", time.Second)
	results, err := client.Search(context.Background(), "黑客帝国 1999")
	if err != nil || len(results) != 1 {
		t.Fatalf("results=%#v err=%v", results, err)
	}
	if results[0].Provider != "下水道" || results[0].TMDBID != "603" {
		t.Fatalf("candidate = %#v", results[0])
	}
}

func TestV3StartDownloadStaleAndFailure(t *testing.T) {
	mode := "stale"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		var envelope struct {
			Method string `json:"method"`
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		body, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(body, &envelope)
		if envelope.Method == "tools/list" {
			writeMCP(w, `{"tools":[{"name":"moviepilot_api"}]}`)
			return
		}
		switch envelope.Params.Arguments["operation_id"] {
		case "search.results":
			if mode == "stale" {
				writeMCPText(w, `{"success":true,"data":{"results":[]}}`)
				return
			}
			writeMCPText(w, `{"success":true,"data":{"results":[{"torrent_info":{"site":2,"site_name":"下水道","enclosure":"https://example.com/dl?1","page_url":"https://example.com/1","title":"Test 1080p","size":1000}}]}}`)
		case "download.add":
			writeMCPText(w, `{"success":false,"message":"任务添加失败"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", time.Second)
	err := client.StartDownload(context.Background(), search.DownloadRequest{Reference: "mpv3:0000000000000000"})
	failure, ok := err.(search.Failure)
	if !ok || failure.Code != "source_stale" || failure.Retryable {
		t.Fatalf("expected source_stale, got %#v", err)
	}

	mode = "fail"
	ref := v3TorrentRef(map[string]any{
		"site":      2,
		"site_name": "下水道",
		"title":     "Test 1080p",
		"enclosure": "https://example.com/dl?1",
		"page_url":  "https://example.com/1",
		"size":      1000,
	})
	err = client.StartDownload(context.Background(), search.DownloadRequest{Reference: ref})
	failure, ok = err.(search.Failure)
	if !ok || failure.Code != "source_unavailable" || !failure.Retryable {
		t.Fatalf("expected retryable source_unavailable, got %#v", err)
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
