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

// ShareReceiver imports a 115 share into an existing destination folder.
type ShareReceiver interface {
	ReceiveShare(ctx context.Context, destinationID, shareCode, receiveCode string, fileIDs []string) error
}

func New(source config.SearchSource, timeout time.Duration, offline Offline, proxyURL *url.URL) search.TransferSource {
	if source.ID == "framehdr" && (source.BaseURL == "" || isFrameHDRHost(source.BaseURL)) {
		if strings.TrimSpace(source.Account) == "" || strings.TrimSpace(source.Token) == "" {
			return nil
		}
		receiver, _ := offline.(ShareReceiver)
		return NewFrameHDR(source.BaseURL, source.Account, source.Token, timeout, receiver, proxyURL)
	}
	if source.ID == "juying" && (source.BaseURL == "" || isJuyingHost(source.BaseURL)) {
		if strings.TrimSpace(source.Account) == "" || strings.TrimSpace(source.Token) == "" {
			return nil
		}
		receiver, _ := offline.(ShareReceiver)
		authMode := juyingSourceAuthMode(source)
		if authMode != "web" && authMode != "developer" {
			return nil
		}
		return NewJuyingWithAuthMode(source.BaseURL, authMode, source.Account, source.Token, timeout, offline, receiver, proxyURL)
	}
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

func isFrameHDRHost(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Port() != "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "framehdr.com" || host == "www.framehdr.com"
}

func isJuyingHost(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Port() != "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "jying.top" || host == "www.jying.top"
}

func juyingSourceAuthMode(source config.SearchSource) string {
	if mode := strings.ToLower(strings.TrimSpace(source.AuthMode)); mode != "" {
		return mode
	}
	if strings.TrimSpace(source.Account) != "" || strings.TrimSpace(source.Token) != "" {
		return "developer"
	}
	return "web"
}
