package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Store struct {
	database *sql.DB
}

type IntegrationRecord struct {
	ID      string
	Label   string
	BaseURL string
}

func Open(ctx context.Context, databasePath string) (*Store, error) {
	if strings.TrimSpace(databasePath) == "" {
		return nil, fmt.Errorf("database path is required")
	}

	if databasePath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(databasePath), 0o700); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}

	dsn, err := sqliteDSN(databasePath)
	if err != nil {
		return nil, fmt.Errorf("build sqlite DSN: %w", err)
	}
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	database.SetMaxOpenConns(8)
	database.SetMaxIdleConns(4)
	database.SetConnMaxIdleTime(5 * time.Minute)

	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	store := &Store{database: database}
	if err := store.migrate(ctx); err != nil {
		_ = database.Close()
		return nil, err
	}
	if databasePath != ":memory:" {
		if err := os.Chmod(databasePath, 0o600); err != nil {
			_ = database.Close()
			return nil, fmt.Errorf("secure database file: %w", err)
		}
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.database.Close()
}

func (s *Store) SyncIntegrations(ctx context.Context, records []IntegrationRecord) error {
	transaction, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin integration sync: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	for _, record := range records {
		if record.ID == "" || record.Label == "" {
			return fmt.Errorf("integration ID and label are required")
		}
		_, err := transaction.ExecContext(ctx, `
			INSERT INTO integrations (id, label, base_url)
			VALUES (?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				label = excluded.label,
				base_url = excluded.base_url,
				updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		`, record.ID, record.Label, record.BaseURL)
		if err != nil {
			return fmt.Errorf("sync integration %q: %w", record.ID, err)
		}
	}

	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit integration sync: %w", err)
	}
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
		)
	`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		if err := s.applyMigration(ctx, entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) applyMigration(ctx context.Context, name string) error {
	versionText, _, ok := strings.Cut(name, "_")
	if !ok {
		return fmt.Errorf("invalid migration name %q", name)
	}
	version, err := strconv.Atoi(versionText)
	if err != nil {
		return fmt.Errorf("invalid migration version %q: %w", name, err)
	}
	migrationSQL, err := migrationFiles.ReadFile("migrations/" + name)
	if err != nil {
		return fmt.Errorf("read migration %q: %w", name, err)
	}

	if version == 17 {
		if _, err := s.database.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
			return fmt.Errorf("disable foreign keys for migration %q: %w", name, err)
		}
		defer func() { _, _ = s.database.ExecContext(ctx, "PRAGMA foreign_keys = ON") }()
	}

	transaction, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %q: %w", name, err)
	}
	defer func() { _ = transaction.Rollback() }()

	var applied bool
	if err := transaction.QueryRowContext(
		ctx,
		"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)",
		version,
	).Scan(&applied); err != nil {
		return fmt.Errorf("check migration %q: %w", name, err)
	}
	if applied {
		return nil
	}

	if _, err := transaction.ExecContext(ctx, string(migrationSQL)); err != nil {
		return fmt.Errorf("apply migration %q: %w", name, err)
	}
	if _, err := transaction.ExecContext(
		ctx,
		"INSERT INTO schema_migrations (version, name) VALUES (?, ?)",
		version,
		name,
	); err != nil {
		return fmt.Errorf("record migration %q: %w", name, err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit migration %q: %w", name, err)
	}
	return nil
}

func sqliteDSN(databasePath string) (string, error) {
	query := url.Values{}
	query.Add("_pragma", "busy_timeout(5000)")
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", "journal_mode(WAL)")
	query.Add("_pragma", "synchronous(NORMAL)")

	if databasePath == ":memory:" {
		query.Set("cache", "shared")
		query.Set("mode", "memory")
		return (&url.URL{Scheme: "file", Opaque: "media-hub", RawQuery: query.Encode()}).String(), nil
	}

	absolutePath, err := filepath.Abs(databasePath)
	if err != nil {
		return "", err
	}
	return (&url.URL{
		Scheme:   "file",
		Path:     filepath.ToSlash(absolutePath),
		RawQuery: query.Encode(),
	}).String(), nil
}
