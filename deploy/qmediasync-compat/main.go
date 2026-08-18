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

const (
	playbackInfoSuffix = "/PlaybackInfo"
	minimalJSONBody    = "{}"
)

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
		if !isPlaybackInfoRequest(request) {
			return
		}
		originalLength := request.ContentLength
		kind := classifyPlaybackInfoLength(originalLength)
		if err := replacePlaybackInfoBody(request); err != nil {
			log.Printf("playbackinfo body=%s rewritten=false prefix=%s err=%v", kind, playbackInfoPrefix(request.URL.Path), err)
			return
		}
		log.Printf(
			"playbackinfo method=%s orig_content_length=%d content_length=%d body=%s rewritten=true prefix=%s",
			request.Method, originalLength, request.ContentLength, kind, playbackInfoPrefix(request.URL.Path),
		)
	}
	proxy.ModifyResponse = func(response *http.Response) error {
		if response.Request != nil && isPlaybackInfoRequest(response.Request) {
			log.Printf("playbackinfo upstream_status=%d content_type=%s", response.StatusCode, response.Header.Get("Content-Type"))
		}
		return nil
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

func isPlaybackInfoRequest(request *http.Request) bool {
	return strings.HasSuffix(request.URL.Path, playbackInfoSuffix)
}

func classifyPlaybackInfoLength(contentLength int64) string {
	switch {
	case contentLength == 0:
		return "empty"
	case contentLength < 0:
		return "unknown"
	case contentLength <= 16:
		return "small"
	default:
		return "other"
	}
}

func replacePlaybackInfoBody(request *http.Request) error {
	if request.Body != nil && request.Body != http.NoBody {
		_, err := io.Copy(io.Discard, io.LimitReader(request.Body, 2<<20))
		closeErr := request.Body.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	request.Method = http.MethodPost
	request.Body = io.NopCloser(bytes.NewBufferString(minimalJSONBody))
	request.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewBufferString(minimalJSONBody)), nil
	}
	request.ContentLength = int64(len(minimalJSONBody))
	request.Trailer = nil
	request.Header.Del("Transfer-Encoding")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Content-Length", "2")
	return nil
}

func playbackInfoPrefix(path string) string {
	if strings.Contains(path, "/emby/") || strings.HasPrefix(path, "/emby") {
		return "emby"
	}
	return "root"
}
