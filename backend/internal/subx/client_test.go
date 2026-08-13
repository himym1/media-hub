package subx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"media-hub/backend/internal/config"
)

func TestClientAuthenticatesAndSanitizesReadResult(t *testing.T) {
	var loginCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			loginCount.Add(1)
			if r.Method != http.MethodPost {
				http.Error(w, "wrong login method", http.StatusMethodNotAllowed)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "provider-token", "token_type": "bearer"})
		case "/api/sources/search":
			if got := r.Header.Get("Authorization"); got != "Bearer provider-token" {
				http.Error(w, "wrong authorization", http.StatusUnauthorized)
				return
			}
			if got := r.URL.Query().Get("keyword"); got != "Van Helsing" {
				http.Error(w, "wrong query", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"title":        "Van Helsing",
				"access_token": "must-not-leak",
				"profile":      map[string]any{"cookie": "must-not-leak"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(config.SubX{BaseURL: server.URL, Username: "admin", Password: "password"}, time.Second)
	result, err := client.Read(context.Background(), "sources.search", Invocation{Query: map[string]string{"keyword": "Van Helsing"}})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if payload["title"] != "Van Helsing" || payload["access_token"] != "[redacted]" {
		t.Fatalf("unexpected result: %#v", payload)
	}
	profile := payload["profile"].(map[string]any)
	if profile["cookie"] != "[redacted]" {
		t.Fatalf("cookie was not redacted: %#v", profile)
	}
	if loginCount.Load() != 1 {
		t.Fatalf("login count = %d, want 1", loginCount.Load())
	}
}

func TestClientConfigurationChangeClearsCachedToken(t *testing.T) {
	var firstLogin atomic.Int32
	var secondLogin atomic.Int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			firstLogin.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "first-token"})
		case "/api/sources/search":
			if r.Header.Get("Authorization") != "Bearer first-token" {
				http.Error(w, "wrong token", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"title":"first"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			secondLogin.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "second-token"})
		case "/api/sources/search":
			if r.Header.Get("Authorization") != "Bearer second-token" {
				http.Error(w, "wrong token", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"title":"second"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer second.Close()

	client := NewClient(config.SubX{BaseURL: first.URL, Username: "admin", Password: "first"}, time.Second)
	if _, err := client.Read(context.Background(), "sources.search", Invocation{Query: map[string]string{"keyword": "test"}}); err != nil {
		t.Fatal(err)
	}
	client.Configure(config.SubX{BaseURL: first.URL, Username: "admin", Password: "first"})
	if _, err := client.Read(context.Background(), "sources.search", Invocation{Query: map[string]string{"keyword": "test"}}); err != nil {
		t.Fatal(err)
	}
	if firstLogin.Load() != 1 {
		t.Fatalf("unchanged configuration caused %d logins", firstLogin.Load())
	}
	client.Configure(config.SubX{BaseURL: second.URL, Username: "admin", Password: "second"})
	if _, err := client.Read(context.Background(), "sources.search", Invocation{Query: map[string]string{"keyword": "test"}}); err != nil {
		t.Fatal(err)
	}
	if firstLogin.Load() != 1 || secondLogin.Load() != 1 {
		t.Fatalf("login counts first=%d second=%d", firstLogin.Load(), secondLogin.Load())
	}
}

func TestInvocationRejectsUnknownParametersAndNativeOperations(t *testing.T) {
	operation, ok := LookupOperation("sources.search")
	if !ok {
		t.Fatal("sources.search missing from fallback catalog")
	}
	if err := validateInvocation(operation, Invocation{Query: map[string]string{"other": "value"}}); err == nil {
		t.Fatal("expected unknown query parameter to fail")
	}
	if _, ok = LookupOperation("archive.run"); ok {
		t.Fatal("native archive operation must not remain delegated")
	}
}
