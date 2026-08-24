package strm

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"media-hub/backend/internal/playback"
)

const RedirectTTL = 50 * time.Minute

type PickCodeResolver interface {
	ResolvePickCode(context.Context, string, string, string) (playback.SourceMedia, error)
}

type cachedRedirect struct {
	url       string
	expiresAt time.Time
}

type RedirectCache struct {
	resolver PickCodeResolver
	ttl      time.Duration
	now      func() time.Time
	mutex    sync.Mutex
	items    map[string]cachedRedirect
}

func NewRedirectCache(resolver PickCodeResolver) *RedirectCache {
	return &RedirectCache{
		resolver: resolver,
		ttl:      RedirectTTL,
		now:      time.Now,
		items:    map[string]cachedRedirect{},
	}
}

func (c *RedirectCache) Resolve(ctx context.Context, pickCode, name, userAgent string) (string, error) {
	if c == nil || c.resolver == nil {
		return "", ErrInvalidRequest
	}
	pickCode = strings.TrimSpace(pickCode)
	if pickCode == "" {
		return "", ErrInvalidRequest
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "video"
	}
	userAgent = strings.TrimSpace(userAgent)
	if userAgent == "" {
		userAgent = playback.PlayerUserAgent
	}
	key := pickCode + "\n" + userAgent
	now := c.now()
	c.mutex.Lock()
	if item, ok := c.items[key]; ok && item.expiresAt.After(now) {
		c.mutex.Unlock()
		return item.url, nil
	}
	c.mutex.Unlock()

	media, err := c.resolver.ResolvePickCode(ctx, pickCode, name, userAgent)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(strings.TrimSpace(media.URL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return "", ErrListFailed
	}
	c.mutex.Lock()
	c.items[key] = cachedRedirect{url: parsed.String(), expiresAt: now.Add(c.ttl)}
	c.mutex.Unlock()
	return parsed.String(), nil
}
