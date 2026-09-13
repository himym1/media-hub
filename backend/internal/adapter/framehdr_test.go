package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"media-hub/backend/internal/search"
)

type memoryShareReceiver struct {
	destination string
	shareCode   string
	receiveCode string
	fileIDs     []string
	err         error
}

func (m *memoryShareReceiver) ReceiveShare(_ context.Context, destinationID, shareCode, receiveCode string, fileIDs []string) error {
	m.destination = destinationID
	m.shareCode = shareCode
	m.receiveCode = receiveCode
	m.fileIDs = append([]string(nil), fileIDs...)
	return m.err
}

func (m *memoryShareReceiver) EnsureFolder(_ context.Context, parentID, name string) (string, error) {
	return parentID + "/" + name, nil
}

type uncertainTestError struct{}

func (uncertainTestError) Error() string             { return "uncertain" }
func (uncertainTestError) SubmissionUncertain() bool { return true }

func TestFrameHDRSearchLogsInAndParsesOnly115Shares(t *testing.T) {
	server, loginCalls := newFrameHDRTestServer(t)
	defer server.Close()
	receiver := &memoryShareReceiver{}
	source := NewFrameHDR(server.URL, "user", "pass", time.Second, receiver, nil)
	source.client.Transport = server.Client().Transport

	results, err := source.Search(context.Background(), "范海辛")
	if err != nil {
		t.Fatal(err)
	}
	if *loginCalls != 1 {
		t.Fatalf("login calls = %d", *loginCalls)
	}
	if len(results) != 2 {
		t.Fatalf("results = %#v", results)
	}
	movie := results[0]
	if movie.Title != "范海辛" || movie.Year != 2004 || movie.MediaType != "movie" || movie.Release.Resolution != "2160p" || movie.Release.VideoCodec != "HEVC" || movie.Release.SizeBytes != 13421772800 {
		t.Fatalf("movie = %+v", movie)
	}
	var reference frameHDRReference
	if err := json.Unmarshal([]byte(movie.SourceRef), &reference); err != nil {
		t.Fatal(err)
	}
	if reference.ShareCode != "shareABC123" || reference.ReceiveCode != "WENG" || strings.Contains(movie.SourceRef, "115cdn.com") {
		t.Fatalf("reference = %+v", reference)
	}
	series := results[1]
	if series.MediaType != "series" || series.Season != 2 || series.EpisodeStart != 3 || series.EpisodeEnd != 5 {
		t.Fatalf("series = %+v", series)
	}
}

func TestFrameHDRTransferReceivesShareWithoutExposingURL(t *testing.T) {
	receiver := &memoryShareReceiver{}
	source := NewFrameHDR("https://framehdr.com", "user", "pass", time.Second, receiver, nil)
	reference, _ := json.Marshal(frameHDRReference{Title: "Van Helsing 2160p", ShareCode: "shareABC123", ReceiveCode: "WENG"})
	result, err := source.StartTransfer(context.Background(), search.TransferRequest{
		Title: "范海辛", Reference: string(reference), DestinationID: "456", IdempotencyKey: "request-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" || result.FileID != "456/范海辛" || result.Path != "范海辛" || receiver.destination != "456/范海辛" || receiver.shareCode != "shareABC123" || receiver.receiveCode != "WENG" || len(receiver.fileIDs) != 0 {
		t.Fatalf("result=%+v receiver=%+v", result, receiver)
	}
}

func TestFrameHDRTransferPropagatesUncertainWrite(t *testing.T) {
	receiver := &memoryShareReceiver{err: uncertainTestError{}}
	source := NewFrameHDR("https://framehdr.com", "user", "pass", time.Second, receiver, nil)
	reference, _ := json.Marshal(frameHDRReference{Title: "Movie", ShareCode: "shareABC123"})
	_, err := source.StartTransfer(context.Background(), search.TransferRequest{Reference: string(reference)})
	var failure search.Failure
	if !errors.As(err, &failure) || failure.Code != "source_submission_unknown" {
		t.Fatalf("error = %#v", err)
	}
}

func TestParseFrameHDRShareURLRejectsNon115AndUnexpectedQuery(t *testing.T) {
	for _, raw := range []string{
		"http://115.com/s/shareABC123",
		"https://evil.example/s/shareABC123",
		"https://115.com/s/shareABC123/extra",
		"https://115.com/s/shareABC123?redirect=https://evil.example",
		"https://115.com:8443/s/shareABC123",
	} {
		if _, _, err := parseFrameHDRShareURL(raw); err == nil {
			t.Fatalf("URL %q was accepted", raw)
		}
	}
}

func TestParseFrameHDRCheckInAction(t *testing.T) {
	action, ok := parseFrameHDRCheckInAction([]byte(`<a id="checkinbtn" href="/attendance.php">签到</a>`))
	if !ok || action.Path != "/attendance.php" || action.Method != http.MethodGet {
		t.Fatalf("action=%#v ok=%v", action, ok)
	}
	action, ok = parseFrameHDRCheckInAction([]byte(`<button id="checkinbtn" onclick="fetch('/user/checkin.php')">签到</button>`))
	if !ok || action.Path != "/user/checkin.php" {
		t.Fatalf("onclick action=%#v ok=%v", action, ok)
	}
}

func TestFrameHDRCheckInCompletesFromAttendancePage(t *testing.T) {
	server, _ := newFrameHDRTestServer(t)
	defer server.Close()
	source := NewFrameHDR(server.URL, "user", "pass", time.Second, nil, nil)
	source.client.Transport = server.Client().Transport
	result, err := source.CheckIn(context.Background())
	if err != nil || result.State != "completed" || result.Message != "签到成功" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestFrameHDRUsesConfiguredProxy(t *testing.T) {
	proxyURL, _ := url.Parse("http://proxy.local:8080")
	source := NewFrameHDR("https://framehdr.com", "user", "pass", time.Second, nil, proxyURL)
	request, _ := http.NewRequest(http.MethodGet, "https://framehdr.com/search.php?q=test", nil)
	transport := source.client.Transport.(*http.Transport)
	resolved, err := transport.Proxy(request)
	if err != nil || resolved.String() != proxyURL.String() {
		t.Fatalf("proxy = %v, err = %v", resolved, err)
	}
}

func newFrameHDRTestServer(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	loginCalls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		loggedIn := false
		if cookie, err := request.Cookie("frame_session"); err == nil && cookie.Value == "ok" {
			loggedIn = true
		}
		loginPage := func() {
			_, _ = w.Write([]byte(`<form id="loginForm"><input type="hidden" name="csrf_token" value="csrf"><input name="username"><input name="password" type="password"></form>`))
		}
		switch {
		case request.URL.Path == "/login.php" && request.Method == http.MethodGet:
			loginPage()
		case request.URL.Path == "/login.php" && request.Method == http.MethodPost:
			loginCalls++
			if request.FormValue("csrf_token") != "csrf" || request.FormValue("login_mode") != "password" || request.FormValue("username") != "user" || request.FormValue("password") != "pass" || request.FormValue("remember_me") != "on" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "frame_session", Value: "ok", Path: "/"})
			http.Redirect(w, request, "/", http.StatusFound)
		case !loggedIn:
			loginPage()
		case request.URL.Path == "/":
			_, _ = w.Write([]byte(`<a href="/user/index.php">用户中心</a><a href="/logout.php">退出</a><a id="checkinbtn" href="/attendance.php">签到</a>`))
		case request.URL.Path == "/user/index.php":
			_, _ = w.Write([]byte(`<a href="/logout.php">退出</a><a id="checkinbtn" href="/attendance.php">签到</a>`))
		case request.URL.Path == "/attendance.php":
			_, _ = w.Write([]byte(`<a href="/logout.php">退出</a><div>签到成功</div>`))
		case request.URL.Path == "/search.php" && request.URL.Query().Get("q") == "范海辛":
			_, _ = w.Write([]byte(`<div class="resource-card" onclick="location.href='detail.php?id=37051'"><span class="category-badge-overlay">电影</span><h3 class="card-title">范海辛 Van Helsing</h3><div class="card-meta"><span class="meta-info">2004 / 美国</span></div></div><div class="resource-card" onclick="location.href='detail.php?id=37052'"><span class="category-badge-overlay">电视剧</span><h3 class="card-title">测试剧 Test Series</h3><span class="meta-info">2025 / 美国</span></div>`))
		case request.URL.Path == "/detail.php" && request.URL.Query().Get("id") == "37051":
			_, _ = w.Write([]byte(`<div class="disk-content" data-disk="115网盘"><div class="download-link" data-link-id="1001"><div class="link-name">Van Helsing 2004 2160p HEVC HDR10 12.5G</div><button onclick="copyToClipboard('https://115cdn.com/s/shareABC123?password=WENG#', '', 1001, '115网盘')">链接</button></div><div class="download-link" data-link-id="1002"><div class="link-name">other cloud</div><button onclick="copyToClipboard('https://www.guangyapan.com/s/not-supported', '', 1002, '光鸭')">链接</button></div><div class="download-link" data-link-id="1003"><button onclick="purchaseLink(1003, 10)">购买</button></div></div>`))
		case request.URL.Path == "/detail.php" && request.URL.Query().Get("id") == "37052":
			_, _ = w.Write([]byte(`<div class="disk-content" data-disk="115"><div class="download-link" data-link-id="2001"><div class="link-description">Test Series S02E03-E05 1080p x264 3G</div><button onclick="copyToClipboard('https://115.com/s/seriesABC123?pwd=A1B2', '', 2001, '115')">链接</button></div></div>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return server, &loginCalls
}
