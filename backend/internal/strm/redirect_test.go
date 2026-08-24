package strm

import (
	"context"
	"testing"
	"time"

	"media-hub/backend/internal/playback"
)

type pickCodeStub struct {
	calls int
	url   string
	err   error
}

func (s *pickCodeStub) ResolvePickCode(context.Context, string, string, string) (playback.SourceMedia, error) {
	s.calls++
	return playback.SourceMedia{URL: s.url}, s.err
}

func TestRedirectCacheReusesUnexpiredURL(t *testing.T) {
	stub := &pickCodeStub{url: "https://cdn.example/video.mkv"}
	cache := NewRedirectCache(stub)
	first, err := cache.Resolve(context.Background(), "pick-1", "video.mkv", "ua")
	if err != nil || first != stub.url || stub.calls != 1 {
		t.Fatalf("first=%q calls=%d err=%v", first, stub.calls, err)
	}
	second, err := cache.Resolve(context.Background(), "pick-1", "video.mkv", "ua")
	if err != nil || second != stub.url || stub.calls != 1 {
		t.Fatalf("cache miss calls=%d err=%v", stub.calls, err)
	}
	cache.now = func() time.Time { return time.Now().Add(RedirectTTL + time.Minute) }
	if _, err := cache.Resolve(context.Background(), "pick-1", "video.mkv", "ua"); err != nil || stub.calls != 2 {
		t.Fatalf("expired cache calls=%d err=%v", stub.calls, err)
	}
}
