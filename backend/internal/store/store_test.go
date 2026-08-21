package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenAppliesMigrationsAndSyncsIntegrations(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	if err := store.SyncIntegrations(ctx, []IntegrationRecord{
		{ID: "emby", Label: "Emby", BaseURL: "http://emby.local"},
	}); err != nil {
		t.Fatalf("sync integrations: %v", err)
	}

	var migrationCount int
	if err := store.database.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if migrationCount != 15 {
		t.Fatalf("migration count = %d, want 15", migrationCount)
	}

	var baseURL string
	if err := store.database.QueryRowContext(
		ctx,
		"SELECT base_url FROM integrations WHERE id = ?",
		"emby",
	).Scan(&baseURL); err != nil {
		t.Fatalf("read integration: %v", err)
	}
	if baseURL != "http://emby.local" {
		t.Fatalf("base URL = %q, want http://emby.local", baseURL)
	}
}
