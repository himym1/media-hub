package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type observedRequest struct {
	method      string
	path        string
	query       string
	body        string
	authorizing string
	host        string
}

func TestEmptyPlaybackInfoRequestGetsMinimalJSONBody(t *testing.T) {
	observed := make(chan observedRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		observed <- observedRequest{
			method: request.Method, path: request.URL.Path, query: request.URL.RawQuery,
			body: string(body), authorizing: request.Header.Get("X-Emby-Token"), host: request.Host,
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"MediaSources":[]}`))
	}))
	defer upstream.Close()

	server := httptest.NewServer(newProxy(mustURL(t, upstream.URL+"/emby")))
	defer server.Close()
	request, err := http.NewRequest(http.MethodGet, server.URL+"/Items/item/PlaybackInfo?MediaSourceId=source", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Emby-Token", "test-token")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()

	got := <-observed
	if got.method != http.MethodPost || got.body != "{}" || got.path != "/emby/Items/item/PlaybackInfo" || got.query != "MediaSourceId=source" || got.authorizing != "test-token" || got.host != strings.TrimPrefix(upstream.URL, "http://") {
		t.Fatalf("unexpected upstream request: %+v", got)
	}
}

func TestExistingPlaybackInfoBodyIsPreserved(t *testing.T) {
	observed := make(chan observedRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		observed <- observedRequest{method: request.Method, body: string(body)}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	server := httptest.NewServer(newProxy(mustURL(t, upstream.URL)))
	defer server.Close()
	body := `{"DeviceProfile":{"Name":"client"}}`
	response, err := http.Post(server.URL+"/Items/item/PlaybackInfo", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()

	got := <-observed
	if got.method != http.MethodPost || got.body != body {
		t.Fatalf("unexpected upstream request: %+v", got)
	}
}

func TestOtherEmptyRequestsRemainUnchanged(t *testing.T) {
	observed := make(chan observedRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		observed <- observedRequest{method: request.Method, path: request.URL.Path, body: string(body)}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	server := httptest.NewServer(newProxy(mustURL(t, upstream.URL)))
	defer server.Close()
	response, err := http.Get(server.URL + "/System/Info")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()

	got := <-observed
	if got.method != http.MethodGet || got.path != "/System/Info" || got.body != "" {
		t.Fatalf("unexpected upstream request: %+v", got)
	}
}

func TestHealthDoesNotReachUpstream(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { upstreamCalls++ }))
	defer upstream.Close()
	server := httptest.NewServer(newProxy(mustURL(t, upstream.URL)))
	defer server.Close()

	response, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || upstreamCalls != 0 {
		t.Fatalf("status=%d upstreamCalls=%d", response.StatusCode, upstreamCalls)
	}
}

func mustURL(t *testing.T, value string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
