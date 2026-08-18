package main

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"
)

const playbackInfoSuffix = "/PlaybackInfo"

func main() {
	upstream, err := parseUpstream(os.Getenv("QMS_EMBY_UPSTREAM"))
	if err != nil {
		log.Fatal(err)
	}
	proxy := newProxy(upstream)
	server := &http.Server{
		Addr:              ":18096",
		Handler:           proxy,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	log.Printf("qmediasync Emby compatibility proxy listening on %s", server.Addr)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func parseUpstream(value string) (*url.URL, error) {
	upstream, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (upstream.Scheme != "http" && upstream.Scheme != "https") || upstream.Host == "" || upstream.User != nil || upstream.RawQuery != "" || upstream.Fragment != "" {
		return nil, errors.New("QMS_EMBY_UPSTREAM must be an http(s) origin without credentials, query, or fragment")
	}
	upstream.Path = strings.TrimRight(upstream.Path, "/")
	return upstream, nil
}

func newProxy(upstream *url.URL) http.Handler {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.ResponseHeaderTimeout = 30 * time.Second
	transport.MaxIdleConns = 32
	transport.MaxIdleConnsPerHost = 16

	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.Transport = transport
	originalDirector := proxy.Director
	proxy.Director = func(request *http.Request) {
		originalDirector(request)
		request.Host = upstream.Host
		if !isEmptyPlaybackInfoRequest(request) {
			return
		}
		request.Method = http.MethodPost
		request.Body = io.NopCloser(bytes.NewBufferString("{}"))
		request.ContentLength = 2
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Content-Length", "2")
	}
	proxy.ErrorHandler = func(writer http.ResponseWriter, _ *http.Request, err error) {
		log.Printf("upstream request failed: %v", err)
		http.Error(writer, "upstream unavailable", http.StatusBadGateway)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("ok\n"))
	})
	mux.Handle("/", proxy)
	return mux
}

func isEmptyPlaybackInfoRequest(request *http.Request) bool {
	return strings.HasSuffix(request.URL.Path, playbackInfoSuffix) && request.ContentLength == 0
}
