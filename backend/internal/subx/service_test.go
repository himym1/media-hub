package subx

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"media-hub/backend/internal/config"
	"media-hub/backend/internal/securepayload"
	"media-hub/backend/internal/store"
)

func TestWorkerLeavesQueuedCommandUntilFallbackIsEnabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/framehdr/save" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer static-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"file_id": "file-1", "path": "/movie", "is_file": true})
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	if _, err := dataStore.EnsureAdmin(ctx, "test-hash"); err != nil {
		t.Fatal(err)
	}
	admin, _, err := dataStore.Admin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	codec, err := securepayload.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(config.SubX{BaseURL: server.URL, Token: "static-token"}, time.Second)
	service := NewService(dataStore, client, codec)
	if err := service.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
		defer waitCancel()
		_ = service.Wait(waitCtx)
	}()

	job, _, err := service.enqueueInternal(ctx, admin.ID, "framehdr.save", "fallback_job", Invocation{Body: json.RawMessage(`{"id":"release-1"}`)})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	queued, err := dataStore.SubXCommand(ctx, admin.ID, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if queued.State != "queued" {
		t.Fatalf("state while disabled = %q", queued.State)
	}

	client.Configure(config.SubX{BaseURL: server.URL, Token: "static-token", SourceEnabled: true})
	service.notify()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		completed, err := dataStore.SubXCommand(ctx, admin.ID, job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if completed.State == "completed" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("queued command did not resume after fallback was enabled")
}
