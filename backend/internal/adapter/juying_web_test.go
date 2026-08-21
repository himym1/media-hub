package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"media-hub/backend/internal/search"
)

func TestJuyingWebSearchDefersAccessUntilTransfer(t *testing.T) {
	var mu sync.Mutex
	loginCalls := 0
	resourceCalls := 0
	accessCalls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/csrf/":
			http.SetCookie(w, &http.Cookie{Name: "csrftoken", Value: "csrf-token", Path: "/"})
			_, _ = w.Write([]byte(`{"status":"success"}`))
		case "/api/app/login/":
			if r.Method != http.MethodPost || r.Header.Get("X-CSRFToken") != "csrf-token" {
				t.Errorf("login request headers = %v", r.Header)
			}
			var body map[string]string
			if json.NewDecoder(r.Body).Decode(&body) != nil || body["username"] != "user" || body["password"] != "pass" {
				t.Errorf("login body keys or values were invalid")
			}
			mu.Lock()
			loginCalls++
			mu.Unlock()
			_, _ = w.Write([]byte(`{"status":"success","token":"user-token"}`))
		case "/api/app/movies/":
			requireJuyingWebToken(t, r, "user-token")
			w.Header().Set("X-Refreshed-Token", "refreshed-token")
			if r.URL.Query().Get("q") != "范海辛" || r.URL.Query().Get("page_size") != "50" {
				t.Errorf("query = %v", r.URL.Query())
			}
			_, _ = w.Write([]byte(`{"status":"success","results":[{"id":20666,"title":"范海辛 Van Helsing","release_year":2004,"movie_type":"movie","tmdb_id":7131}]}`))
		case "/api/app/movie/20666/resources/":
			requireJuyingWebToken(t, r, "refreshed-token")
			mu.Lock()
			resourceCalls++
			ticket := "search-ticket"
			if resourceCalls == 2 {
				ticket = "transfer-ticket"
			}
			mu.Unlock()
			_, _ = w.Write([]byte(`{"status":"success","has_more":false,"resources":[` +
				`{"id":901,"resource_type":"115","resource_description":"Van.Helsing.2004.2160p.HEVC","file_size":13421772800,"link_exposed":true,"access_ticket":"` + ticket + `","access_endpoint":"/api/app/resource/901/access/","access_mode":"open","share_link":""},` +
				`{"id":902,"resource_type":"115","resource_description":"hidden","link_exposed":false,"access_ticket":"","access_endpoint":""},` +
				`{"id":903,"resource_type":"115","resource_description":"cross-origin","link_exposed":true,"access_ticket":"other-ticket","access_endpoint":"https://example.invalid/access/"}` +
				`]}`))
		case "/api/app/resource/901/access/":
			requireJuyingWebToken(t, r, "refreshed-token")
			if r.Method != http.MethodPost || r.Header.Get("X-CSRFToken") != "csrf-token" {
				t.Errorf("access request headers = %v", r.Header)
			}
			var body map[string]string
			if json.NewDecoder(r.Body).Decode(&body) != nil || body["access_ticket"] != "transfer-ticket" {
				t.Errorf("access payload was invalid")
			}
			mu.Lock()
			accessCalls++
			mu.Unlock()
			_, _ = w.Write([]byte(`{"status":"success","target":"https://115.com/s/shareABC123","access_code":"WENG","access_mode":"open","expires_in":30}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	target := &juyingTransferTarget{}
	source := NewJuyingWithAuthMode(server.URL, "web", "user", "pass", time.Second, target, target, nil)
	source.client.Transport = server.Client().Transport
	results, err := source.Search(context.Background(), "范海辛")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "范海辛 Van Helsing" || results[0].TMDBID != "7131" ||
		results[0].ReleaseTitle != "Van.Helsing.2004.2160p.HEVC" || results[0].TransferState != "available" ||
		results[0].Release.Resolution != "2160p" || results[0].Release.SizeBytes != 13421772800 {
		t.Fatalf("results = %#v", results)
	}
	if strings.Contains(results[0].SourceRef, "search-ticket") || strings.Contains(results[0].SourceRef, "115.com") || strings.Contains(results[0].SourceRef, "refreshed-token") || strings.Contains(results[0].SourceRef, "pass") {
		t.Fatalf("reference contains session or upstream secret: %s", results[0].SourceRef)
	}
	mu.Lock()
	if accessCalls != 0 {
		t.Fatalf("search called access endpoint %d times", accessCalls)
	}
	mu.Unlock()
	if _, err := source.StartTransfer(context.Background(), search.TransferRequest{
		Title: "Van Helsing", Reference: results[0].SourceRef, DestinationID: "dest", IdempotencyKey: "op",
	}); err != nil {
		t.Fatal(err)
	}
	if target.destinationID != "folder-1" || target.shareCode != "shareABC123" || target.receiveCode != "WENG" || len(target.magnets) != 0 {
		t.Fatalf("target = %+v", target)
	}
	mu.Lock()
	defer mu.Unlock()
	if loginCalls != 1 || resourceCalls != 2 || accessCalls != 1 {
		t.Fatalf("calls login=%d resources=%d access=%d", loginCalls, resourceCalls, accessCalls)
	}
}

func TestJuyingWebRejectsChangedResourceIdentityBeforeAccess(t *testing.T) {
	resourceCalls := 0
	accessCalls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/csrf/":
			http.SetCookie(w, &http.Cookie{Name: "csrftoken", Value: "csrf-token", Path: "/"})
			_, _ = w.Write([]byte(`{}`))
		case "/api/app/login/":
			_, _ = w.Write([]byte(`{"token":"user-token"}`))
		case "/api/app/movies/":
			_, _ = w.Write([]byte(`{"status":"success","results":[{"id":1,"title":"Van Helsing","release_year":2004,"movie_type":"movie","tmdb_id":7131}]}`))
		case "/api/app/movie/1/resources/":
			resourceCalls++
			title := "Van.Helsing.2004.2160p"
			if resourceCalls > 1 {
				title = "Wrong.Movie.2020.2160p"
			}
			_, _ = w.Write([]byte(`{"status":"success","has_more":false,"resources":[{"id":2,"resource_type":"115","title":"` + title + `","link_exposed":true,"access_ticket":"ticket","access_endpoint":"/api/app/resource/2/access/"}]}`))
		case "/api/app/resource/2/access/":
			accessCalls++
			_, _ = w.Write([]byte(`{"status":"success","target":"magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=Wrong.Movie.2020.2160p"}`))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	target := &juyingTransferTarget{}
	source := NewJuyingWithAuthMode(server.URL, "web", "user", "pass", time.Second, target, target, nil)
	source.client.Transport = server.Client().Transport
	results, err := source.Search(context.Background(), "Van Helsing")
	if err != nil || len(results) != 1 {
		t.Fatalf("results=%#v err=%v", results, err)
	}
	_, err = source.StartTransfer(context.Background(), search.TransferRequest{
		Reference: results[0].SourceRef, DestinationID: "dest", IdempotencyKey: "op",
	})
	var failure search.Failure
	if !errors.As(err, &failure) || failure.Code != "source_identity_mismatch" || failure.Retryable {
		t.Fatalf("failure = %#v", err)
	}
	if accessCalls != 0 || len(target.magnets) != 0 {
		t.Fatalf("access=%d target=%+v", accessCalls, target)
	}
}

func TestJuyingWebMagnetCandidateIsNotTransferable(t *testing.T) {
	candidate, ok := juyingWebCandidate("1", "Van Helsing", 2004, "movie", juyingResource{
		ID: json.RawMessage("2"), ResourceType: "magnet", Title: "Van.Helsing.2004.2160p",
		LinkExposed: true, AccessTicket: "ticket", AccessEndpoint: "/api/app/resource/2/access/",
	})
	if !ok {
		t.Fatal("candidate was rejected")
	}
	if candidate.SourceRef != "" || candidate.TransferState != "unavailable" {
		t.Fatalf("candidate remained transferable: %+v", candidate)
	}
}

func TestJuyingWebRelogsOnceAfterUnauthorized(t *testing.T) {
	var mu sync.Mutex
	loginCalls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/csrf/":
			http.SetCookie(w, &http.Cookie{Name: "csrftoken", Value: "csrf-token", Path: "/"})
			_, _ = w.Write([]byte(`{}`))
		case "/api/app/login/":
			mu.Lock()
			loginCalls++
			token := "expired-token"
			if loginCalls == 2 {
				token = "fresh-token"
			}
			mu.Unlock()
			_, _ = w.Write([]byte(`{"token":"` + token + `"}`))
		case "/api/app/movies/":
			if r.Header.Get("X-App-User-Token") == "expired-token" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"status":"error"}`))
				return
			}
			requireJuyingWebToken(t, r, "fresh-token")
			_, _ = w.Write([]byte(`{"status":"success","results":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	source := NewJuyingWithAuthMode(server.URL, "web", "user", "pass", time.Second, nil, nil, nil)
	source.client.Transport = server.Client().Transport
	results, err := source.Search(context.Background(), "test")
	if err != nil || len(results) != 0 {
		t.Fatalf("results=%#v err=%v", results, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if loginCalls != 2 {
		t.Fatalf("login calls = %d", loginCalls)
	}
}

func TestJuyingWebRateLimitDoesNotReachReceiver(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/csrf/":
			http.SetCookie(w, &http.Cookie{Name: "csrftoken", Value: "csrf-token", Path: "/"})
			_, _ = w.Write([]byte(`{}`))
		case "/api/app/login/":
			_, _ = w.Write([]byte(`{"token":"user-token"}`))
		case "/api/app/movies/":
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"code":"rate_limited"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	target := &juyingTransferTarget{}
	source := NewJuyingWithAuthMode(server.URL, "web", "user", "pass", time.Second, target, target, nil)
	source.client.Transport = server.Client().Transport
	_, err := source.Search(context.Background(), "test")
	var failure search.Failure
	if err == nil || !errors.As(err, &failure) || !failure.Retryable || failure.Code != "source_rate_limited" || failure.Message != "聚影请求过于频繁" {
		t.Fatalf("err = %#v", err)
	}
	if target.shareCode != "" || len(target.magnets) != 0 {
		t.Fatalf("target was called: %+v", target)
	}
}

func TestJuyingAccessUnknownRequiresConfirmation(t *testing.T) {
	err := juyingAccessFailure(search.Failure{Code: "source_unavailable", Message: "invalid response", Retryable: true})
	var failure search.Failure
	if !errors.As(err, &failure) || failure.Code != "source_access_unknown" || !failure.Retryable {
		t.Fatalf("failure = %#v", err)
	}
	rateLimited := search.Failure{Code: "source_rate_limited", Message: "rate limited", Retryable: true}
	if got := juyingAccessFailure(rateLimited); !errors.Is(got, rateLimited) {
		t.Fatalf("rate limit changed: %#v", got)
	}
}

func TestJuyingWebRejectsForgedReferenceBeforeNetwork(t *testing.T) {
	target := &juyingTransferTarget{}
	source := NewJuyingWithAuthMode("https://www.jying.top", "web", "user", "pass", time.Second, target, target, nil)
	_, err := source.StartTransfer(context.Background(), search.TransferRequest{
		Reference:     `{"kind":"web","title":"Movie","movieId":"../1","resourceId":"2"}`,
		DestinationID: "dest", IdempotencyKey: "op",
	})
	if err == nil {
		t.Fatal("forged reference was accepted")
	}
	if target.shareCode != "" || len(target.magnets) != 0 {
		t.Fatalf("target was called: %+v", target)
	}
}

func TestJuyingWebLoginTokenAcceptsAlternateShapes(t *testing.T) {
	header := make(http.Header)
	header.Set("X-App-User-Token", "header-token")
	cases := []struct {
		body string
		want string
	}{
		{`{"status":"success","token":"user-token"}`, "user-token"},
		{`{"user_token":"nested-user"}`, "nested-user"},
		{`{"data":{"access_token":"inner-token"}}`, "inner-token"},
		{`{"status":"success"}`, "header-token"},
		{`not-json`, "header-token"},
		{`{"token":""}`, "header-token"},
	}
	for _, test := range cases {
		if got := juyingWebLoginToken([]byte(test.body), header); got != test.want {
			t.Fatalf("body %s: got %q want %q", test.body, got, test.want)
		}
	}
	if got := juyingWebLoginToken([]byte(`{}`), nil); got != "" {
		t.Fatalf("empty login produced %q", got)
	}
}

func TestJuyingWebCheckInUsesStatsAndDo(t *testing.T) {
	doCalls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/csrf/":
			http.SetCookie(w, &http.Cookie{Name: "csrftoken", Value: "csrf-token", Path: "/"})
			_, _ = w.Write([]byte(`{"status":"success"}`))
		case "/api/app/login/":
			_, _ = w.Write([]byte(`{"status":"success","token":"user-token"}`))
		case "/api/app/checkin/stats/":
			requireJuyingWebToken(t, r, "user-token")
			_, _ = w.Write([]byte(`{"status":"success","checked_today":false,"my_total_days":12}`))
		case "/api/app/checkin/do/":
			if r.Method != http.MethodPost {
				t.Errorf("method = %s", r.Method)
			}
			requireJuyingWebToken(t, r, "user-token")
			doCalls++
			_, _ = w.Write([]byte(`{"status":"success","points":5}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	source := NewJuyingWithAuthMode(server.URL, "web", "user", "pass", time.Second, nil, nil, nil)
	source.client.Transport = server.Client().Transport
	result, err := source.CheckIn(context.Background())
	if err != nil || result.State != "completed" || result.Message != "签到成功" || doCalls != 1 {
		t.Fatalf("result=%#v doCalls=%d err=%v", result, doCalls, err)
	}
}

func TestJuyingWebCheckInSkipsWhenAlreadyDone(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/csrf/":
			http.SetCookie(w, &http.Cookie{Name: "csrftoken", Value: "csrf-token", Path: "/"})
			_, _ = w.Write([]byte(`{"status":"success"}`))
		case "/api/app/login/":
			_, _ = w.Write([]byte(`{"status":"success","token":"user-token"}`))
		case "/api/app/checkin/stats/":
			_, _ = w.Write([]byte(`{"status":"success","checked_today":true}`))
		case "/api/app/checkin/do/":
			t.Fatal("already checked-in account posted again")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	source := NewJuyingWithAuthMode(server.URL, "web", "user", "pass", time.Second, nil, nil, nil)
	source.client.Transport = server.Client().Transport
	result, err := source.CheckIn(context.Background())
	if err != nil || result.State != "completed" || result.Message != "今日已签到" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestJuyingDeveloperCheckInIsSkipped(t *testing.T) {
	source := NewJuyingWithAuthMode("https://www.jying.top", "developer", "app", "key", time.Second, nil, nil, nil)
	result, err := source.CheckIn(context.Background())
	if err != nil || result.State != "skipped" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func requireJuyingWebToken(t *testing.T, request *http.Request, expected string) {
	t.Helper()
	if request.Header.Get("X-App-User-Token") != expected || request.Header.Get("X-Requested-With") != "XMLHttpRequest" {
		t.Errorf("web session headers were not set")
	}
}
