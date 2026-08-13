package adapter

import (
	"context"
	"net/url"
	"strings"
	"time"

	"media-hub/backend/internal/config"
	"media-hub/backend/internal/search"
)

// Offline submits magnet or torrent URLs to 115 offline download.
type Offline interface {
	AddOfflineURLs(ctx context.Context, destinationID string, urls []string) error
}

func New(source config.SearchSource, timeout time.Duration, offline Offline, proxyURL *url.URL) search.TransferSource {
	if source.ID == "mikan" && (source.BaseURL == "" || isMikanHost(source.BaseURL)) {
		return NewMikan(source.BaseURL, source.Token, timeout, offline, proxyURL)
	}
	if source.ID == "sidhub" && (source.BaseURL == "" || isSidhubHost(source.BaseURL)) {
		return NewSidhub(source.BaseURL, timeout, offline, proxyURL)
	}
	if source.BaseURL == "" {
		return nil
	}
	return search.NewHTTPSource(source.ID, source.Label, source.BaseURL, source.Token, timeout)
}

func isMikanHost(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "mikanani.me" || host == "www.mikanani.me" || strings.HasSuffix(host, ".mikanani.me")
}

func isSidhubHost(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Port() != "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "sidhub.cc" || host == "www.sidhub.cc" || host == "seeduck.cc" || host == "www.seeduck.cc"
}
