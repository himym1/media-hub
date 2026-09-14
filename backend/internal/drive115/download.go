package drive115

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"media-hub/backend/internal/playback"
)

const maxSubtitleBytes = 8 << 20

func (s *AuthService) DownloadFile(ctx context.Context, pickCode, name string) ([]byte, error) {
	media, err := s.ResolvePickCode(ctx, pickCode, name, playback.PlayerUserAgent)
	if err != nil {
		return nil, err
	}
	return downloadHTTPS(ctx, media.URL, playback.PlayerUserAgent)
}

func downloadHTTPS(ctx context.Context, rawURL, userAgent string) ([]byte, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("subtitle download url is invalid")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", userAgent)
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("subtitle download was rejected")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxSubtitleBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) == 0 || len(body) > maxSubtitleBytes {
		return nil, fmt.Errorf("subtitle download is empty or too large")
	}
	return body, nil
}
