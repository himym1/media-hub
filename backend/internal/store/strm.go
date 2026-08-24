package store

import (
	"context"
	"database/sql"
)

type STRMSyncRun struct {
	ID        int64
	MediaType string
	Full      bool
	Scanned   int
	Created   int
	Updated   int
	Skipped   int
	Removed   int
	Error     string
	StartedAt int64
	Finished  int64
}

func (s *Store) STRMFolderStates(ctx context.Context, targetPath string) (map[string]int64, error) {
	rows, err := s.database.QueryContext(ctx, `
		SELECT folder_id, updated_at FROM strm_folder_states WHERE target_path = ?
	`, targetPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]int64{}
	for rows.Next() {
		var folderID string
		var updatedAt int64
		if err := rows.Scan(&folderID, &updatedAt); err != nil {
			return nil, err
		}
		result[folderID] = updatedAt
	}
	return result, rows.Err()
}

func (s *Store) ReplaceSTRMFolderStates(ctx context.Context, targetPath string, states map[string]int64, merge bool) error {
	transaction, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback() }()
	if !merge {
		if _, err := transaction.ExecContext(ctx, `DELETE FROM strm_folder_states WHERE target_path = ?`, targetPath); err != nil {
			return err
		}
	}
	for folderID, updatedAt := range states {
		if _, err := transaction.ExecContext(ctx, `
			INSERT INTO strm_folder_states (folder_id, target_path, updated_at)
			VALUES (?, ?, ?)
			ON CONFLICT(folder_id, target_path) DO UPDATE SET updated_at = excluded.updated_at
		`, folderID, targetPath, updatedAt); err != nil {
			return err
		}
	}
	return transaction.Commit()
}

func (s *Store) InsertSTRMSyncRun(ctx context.Context, run STRMSyncRun) error {
	_, err := s.database.ExecContext(ctx, `
		INSERT INTO strm_sync_runs (
			media_type, full_sync, scanned, created_count, updated_count, skipped, removed, error, started_at, finished_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, run.MediaType, boolToInt(run.Full), run.Scanned, run.Created, run.Updated, run.Skipped, run.Removed, run.Error, run.StartedAt, run.Finished)
	return err
}

func (s *Store) LatestSTRMSyncRun(ctx context.Context) (STRMSyncRun, bool, error) {
	var run STRMSyncRun
	var full int
	err := s.database.QueryRowContext(ctx, `
		SELECT id, media_type, full_sync, scanned, created_count, updated_count, skipped, removed, error, started_at, finished_at
		FROM strm_sync_runs ORDER BY id DESC LIMIT 1
	`).Scan(&run.ID, &run.MediaType, &full, &run.Scanned, &run.Created, &run.Updated, &run.Skipped, &run.Removed, &run.Error, &run.StartedAt, &run.Finished)
	if err == sql.ErrNoRows {
		return STRMSyncRun{}, false, nil
	}
	if err != nil {
		return STRMSyncRun{}, false, err
	}
	run.Full = full == 1
	return run, true, nil
}

func (s *Store) LatestFullSTRMSyncAt(ctx context.Context) (int64, error) {
	var value sql.NullInt64
	err := s.database.QueryRowContext(ctx, `
		SELECT MAX(finished_at) FROM strm_sync_runs WHERE full_sync = 1 AND error = ''
	`).Scan(&value)
	if err != nil {
		return 0, err
	}
	if !value.Valid {
		return 0, nil
	}
	return value.Int64, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
