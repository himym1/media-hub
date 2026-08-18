package emby

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"media-hub/backend/internal/integration"
)

func TestPlaybackCheckerUsesImmutableFacadeAndCurrentCredentials(t *testing.T) {
	facade := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/System/Info" || request.Header.Get("X-Emby-Token") != "new-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"Id":"server-1","ServerName":"Emby","Version":"4.9"}`))
	}))
	defer facade.Close()

	client := NewClientWithPlayback("https://emby.example", "old-key", time.Second, "", facade.URL)
	client.Configure("https://emby-new.example", "new-key", "")
	health := NewPlaybackChecker(client).Check(context.Background())
	if health.ID != "emby-playback" || health.Status != integration.StatusHealthy {
		t.Fatalf("health = %#v", health)
	}
}

func TestPlaybackCheckerDistinguishesUnconfiguredAndUnauthorized(t *testing.T) {
	unconfigured := NewPlaybackChecker(NewClient("https://emby.example", "key", time.Second)).Check(context.Background())
	if unconfigured.Status != integration.StatusUnconfigured {
		t.Fatalf("unconfigured = %#v", unconfigured)
	}

	facade := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer facade.Close()
	unauthorized := NewPlaybackChecker(
		NewClientWithPlayback("https://emby.example", "bad-key", time.Second, "", facade.URL),
	).Check(context.Background())
	if unauthorized.Status != integration.StatusDegraded {
		t.Fatalf("unauthorized = %#v", unauthorized)
	}
}
