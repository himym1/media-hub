package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrCheckInNotFound     = errors.New("source check-in not found")
	ErrCheckInNotRetryable = errors.New("source check-in is not retryable")
)

type SourceCheckIn struct {
	SourceID      string
	Label         string
	State         string
	Message       string
	ErrorCode     string
	Retryable     bool
	Attempts      int
	NextAttemptAt int64
	LastSuccessAt int64
	LastDay       string
	NotifiedDay   string
	NotifiedState string
	UpdatedAt     int64
}

const sourceCheckInColumns = `
	source_id, label, state, message, error_code, retryable, attempts,
	next_attempt_at, last_success_at, last_day, notified_day, notified_state, updated_at`

func (s *Store) ListSourceCheckIns(ctx context.Context) ([]SourceCheckIn, error) {
	rows, err := s.database.QueryContext(ctx, `SELECT `+sourceCheckInColumns+` FROM source_checkins ORDER BY source_id`)
	if err != nil {
		return nil, fmt.Errorf("list source check-ins: %w", err)
	}
	defer rows.Close()
	items := make([]SourceCheckIn, 0)
	for rows.Next() {
		item, err := scanSourceCheckIn(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SourceCheckIn(ctx context.Context, sourceID string) (SourceCheckIn, error) {
	item, err := scanSourceCheckIn(s.database.QueryRowContext(ctx, `SELECT `+sourceCheckInColumns+` FROM source_checkins WHERE source_id = ?`, sourceID))
	if errors.Is(err, sql.ErrNoRows) {
		return SourceCheckIn{}, ErrCheckInNotFound
	}
	if err != nil {
		return SourceCheckIn{}, fmt.Errorf("read source check-in: %w", err)
	}
	return item, nil
}

func (s *Store) RecoverInterruptedCheckIns(ctx context.Context, now time.Time) error {
	_, err := s.database.ExecContext(ctx, `
		UPDATE source_checkins
		SET state = 'failed', error_code = 'interrupted', message = '服务重启时签到结果未知', retryable = 1, next_attempt_at = 0, updated_at = ?
		WHERE state = 'running'`, now.UTC().Unix())
	if err != nil {
		return fmt.Errorf("recover interrupted source check-ins: %w", err)
	}
	return nil
}

func (s *Store) ClaimSourceCheckIn(ctx context.Context, sourceID, label, day string, now time.Time, force bool) (SourceCheckIn, bool, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return SourceCheckIn{}, false, fmt.Errorf("begin source check-in claim: %w", err)
	}
	defer tx.Rollback()

	item, err := scanSourceCheckIn(tx.QueryRowContext(ctx, `SELECT `+sourceCheckInColumns+` FROM source_checkins WHERE source_id = ?`, sourceID))
	if errors.Is(err, sql.ErrNoRows) {
		created := SourceCheckIn{
			SourceID: sourceID, Label: label, State: "running", UpdatedAt: now.UTC().Unix(), Attempts: 1,
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO source_checkins (
				source_id, label, state, message, error_code, retryable, attempts,
				next_attempt_at, last_success_at, last_day, notified_day, notified_state, updated_at
			) VALUES (?, ?, 'running', '', '', 0, 1, 0, 0, '', '', '', ?)`,
			sourceID, label, created.UpdatedAt); err != nil {
			return SourceCheckIn{}, false, fmt.Errorf("insert source check-in: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return SourceCheckIn{}, false, fmt.Errorf("commit source check-in claim: %w", err)
		}
		return created, true, nil
	}
	if err != nil {
		return SourceCheckIn{}, false, fmt.Errorf("read source check-in: %w", err)
	}
	if !force {
		if item.State == "running" {
			return item, false, nil
		}
		if item.LastDay == day {
			switch item.State {
			case "completed", "skipped", "needs_attention":
				return item, false, nil
			case "failed":
				if !item.Retryable || item.NextAttemptAt > now.UTC().Unix() {
					return item, false, nil
				}
			}
		} else if item.NextAttemptAt > now.UTC().Unix() {
			return item, false, nil
		}
	}
	item.Label = label
	item.State = "running"
	item.Message = ""
	item.ErrorCode = ""
	item.Retryable = false
	item.Attempts++
	item.NextAttemptAt = 0
	item.UpdatedAt = now.UTC().Unix()
	if _, err := tx.ExecContext(ctx, `
		UPDATE source_checkins SET
			label = ?, state = 'running', message = '', error_code = '', retryable = 0,
			attempts = ?, next_attempt_at = 0, updated_at = ?
		WHERE source_id = ?`, label, item.Attempts, item.UpdatedAt, sourceID); err != nil {
		return SourceCheckIn{}, false, fmt.Errorf("claim source check-in: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return SourceCheckIn{}, false, fmt.Errorf("commit source check-in claim: %w", err)
	}
	return item, true, nil
}

func (s *Store) FinishSourceCheckIn(ctx context.Context, item SourceCheckIn) error {
	_, err := s.database.ExecContext(ctx, `
		UPDATE source_checkins SET
			label = ?, state = ?, message = ?, error_code = ?, retryable = ?,
			attempts = ?, next_attempt_at = ?, last_success_at = ?, last_day = ?,
			notified_day = ?, notified_state = ?, updated_at = ?
		WHERE source_id = ? AND state = 'running'`,
		item.Label, item.State, item.Message, item.ErrorCode, item.Retryable,
		item.Attempts, item.NextAttemptAt, item.LastSuccessAt, item.LastDay,
		item.NotifiedDay, item.NotifiedState, item.UpdatedAt, item.SourceID)
	if err != nil {
		return fmt.Errorf("finish source check-in: %w", err)
	}
	return nil
}

func (s *Store) MarkSourceCheckInNotified(ctx context.Context, sourceID, day, state string, now time.Time) error {
	_, err := s.database.ExecContext(ctx, `
		UPDATE source_checkins SET notified_day = ?, notified_state = ?, updated_at = ? WHERE source_id = ?`,
		day, state, now.UTC().Unix(), sourceID)
	if err != nil {
		return fmt.Errorf("mark source check-in notified: %w", err)
	}
	return nil
}

func (s *Store) RetrySourceCheckIn(ctx context.Context, sourceID string, now time.Time) (SourceCheckIn, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return SourceCheckIn{}, fmt.Errorf("begin source check-in retry: %w", err)
	}
	defer tx.Rollback()
	item, err := scanSourceCheckIn(tx.QueryRowContext(ctx, `SELECT `+sourceCheckInColumns+` FROM source_checkins WHERE source_id = ?`, sourceID))
	if errors.Is(err, sql.ErrNoRows) {
		return SourceCheckIn{}, ErrCheckInNotFound
	}
	if err != nil {
		return SourceCheckIn{}, fmt.Errorf("read source check-in: %w", err)
	}
	if item.State == "running" {
		return SourceCheckIn{}, ErrCheckInNotRetryable
	}
	item.State = "idle"
	item.Message = ""
	item.ErrorCode = ""
	item.Retryable = false
	item.Attempts = 0
	item.NextAttemptAt = 0
	item.LastDay = ""
	item.UpdatedAt = now.UTC().Unix()
	if _, err := tx.ExecContext(ctx, `
		UPDATE source_checkins SET
			state = 'idle', message = '', error_code = '', retryable = 0, attempts = 0,
			next_attempt_at = 0, last_day = '', notified_day = '', notified_state = '', updated_at = ?
		WHERE source_id = ?`, item.UpdatedAt, sourceID); err != nil {
		return SourceCheckIn{}, fmt.Errorf("retry source check-in: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return SourceCheckIn{}, fmt.Errorf("commit source check-in retry: %w", err)
	}
	return item, nil
}

func scanSourceCheckIn(row interface{ Scan(dest ...any) error }) (SourceCheckIn, error) {
	var item SourceCheckIn
	err := row.Scan(
		&item.SourceID, &item.Label, &item.State, &item.Message, &item.ErrorCode, &item.Retryable,
		&item.Attempts, &item.NextAttemptAt, &item.LastSuccessAt, &item.LastDay, &item.NotifiedDay, &item.NotifiedState, &item.UpdatedAt,
	)
	return item, err
}
