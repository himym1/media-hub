package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrNotificationNotFound     = errors.New("transfer notification not found")
	ErrNotificationNotRetryable = errors.New("transfer notification is not retryable")
)

type TransferNotification struct {
	JobID        string
	EventType    string
	JobState     string
	Title        string
	ErrorMessage string
	State        string
	Attempts     int
	NextAttempt  int64
	CreatedAt    int64
	UpdatedAt    int64
}

func (s *Store) EnsureTerminalNotifications(ctx context.Context, now time.Time) error {
	_, err := s.database.ExecContext(ctx, `
		INSERT INTO transfer_notifications (
			job_id, event_type, state, attempts, next_attempt_at, created_at, updated_at
		)
		SELECT id, state, 'pending', 0, 0, ?, ?
		FROM transfer_jobs
		WHERE state IN ('completed', 'failed', 'needs_attention')
		ON CONFLICT(job_id, event_type) DO NOTHING`, now.UTC().Unix(), now.UTC().Unix())
	if err != nil {
		return fmt.Errorf("ensure terminal notifications: %w", err)
	}
	return nil
}

func (s *Store) MarkInterruptedNotifications(ctx context.Context, now time.Time) error {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin notification recovery: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT job_id, event_type FROM transfer_notifications WHERE state = 'submitting'`)
	if err != nil {
		return fmt.Errorf("list interrupted notifications: %w", err)
	}
	var interrupted [][2]string
	for rows.Next() {
		var item [2]string
		if err := rows.Scan(&item[0], &item[1]); err != nil {
			rows.Close()
			return fmt.Errorf("scan interrupted notification: %w", err)
		}
		interrupted = append(interrupted, item)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range interrupted {
		if _, err := tx.ExecContext(ctx, `
			UPDATE transfer_notifications SET state = 'needs_attention', updated_at = ?
			WHERE job_id = ? AND event_type = ? AND state = 'submitting'`, now.UTC().Unix(), item[0], item[1]); err != nil {
			return fmt.Errorf("recover interrupted notification: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO transfer_job_events (job_id, state, message, created_at)
			SELECT id, state, '服务重启时企业微信通知结果未知', ? FROM transfer_jobs WHERE id = ?`, now.UTC().Unix(), item[0]); err != nil {
			return fmt.Errorf("record interrupted notification: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit notification recovery: %w", err)
	}
	return nil
}

func (s *Store) NextTransferNotification(ctx context.Context, now time.Time) (TransferNotification, bool, error) {
	var item TransferNotification
	err := s.database.QueryRowContext(ctx, `
		SELECT n.job_id, n.event_type, j.state, j.title, j.error_message, n.state, n.attempts, n.next_attempt_at
		FROM transfer_notifications n
		JOIN transfer_jobs j ON j.id = n.job_id
		WHERE n.state = 'pending' AND n.next_attempt_at <= ?
		ORDER BY n.created_at LIMIT 1`, now.UTC().Unix()).Scan(
		&item.JobID, &item.EventType, &item.JobState, &item.Title, &item.ErrorMessage,
		&item.State, &item.Attempts, &item.NextAttempt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return TransferNotification{}, false, nil
	}
	if err != nil {
		return TransferNotification{}, false, fmt.Errorf("read next notification: %w", err)
	}
	return item, true, nil
}

func (s *Store) BeginTransferNotification(ctx context.Context, item TransferNotification, now time.Time) (bool, error) {
	result, err := s.database.ExecContext(ctx, `
		UPDATE transfer_notifications SET state = 'submitting', attempts = attempts + 1, updated_at = ?
		WHERE job_id = ? AND event_type = ? AND state = 'pending'`,
		now.UTC().Unix(), item.JobID, item.EventType)
	if err != nil {
		return false, fmt.Errorf("begin transfer notification: %w", err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (s *Store) RetryTransferNotification(ctx context.Context, item TransferNotification, next, now time.Time) error {
	_, err := s.database.ExecContext(ctx, `
		UPDATE transfer_notifications SET state = 'pending', next_attempt_at = ?, updated_at = ?
		WHERE job_id = ? AND event_type = ? AND state = 'submitting'`,
		next.UTC().Unix(), now.UTC().Unix(), item.JobID, item.EventType)
	if err != nil {
		return fmt.Errorf("schedule transfer notification retry: %w", err)
	}
	return nil
}

func (s *Store) FinishTransferNotification(
	ctx context.Context,
	item TransferNotification,
	state, eventMessage string,
	now time.Time,
) error {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transfer notification completion: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		UPDATE transfer_notifications SET state = ?, next_attempt_at = 0, updated_at = ?
		WHERE job_id = ? AND event_type = ? AND state = 'submitting'`,
		state, now.UTC().Unix(), item.JobID, item.EventType); err != nil {
		return fmt.Errorf("finish transfer notification: %w", err)
	}
	if eventMessage != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO transfer_job_events (job_id, state, message, created_at)
			SELECT id, state, ?, ? FROM transfer_jobs WHERE id = ?`, eventMessage, now.UTC().Unix(), item.JobID); err != nil {
			return fmt.Errorf("record transfer notification: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transfer notification completion: %w", err)
	}
	return nil
}

func (s *Store) ListTransferNotifications(ctx context.Context, userID int64, limit int) ([]TransferNotification, error) {
	rows, err := s.database.QueryContext(ctx, `
		SELECT n.job_id, n.event_type, j.state, j.title, n.state, n.attempts, n.next_attempt_at, n.created_at, n.updated_at
		FROM transfer_notifications n
		JOIN transfer_jobs j ON j.id = n.job_id
		WHERE j.user_id = ?
		ORDER BY n.created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list transfer notifications: %w", err)
	}
	defer rows.Close()
	items := make([]TransferNotification, 0)
	for rows.Next() {
		var item TransferNotification
		if err := rows.Scan(
			&item.JobID, &item.EventType, &item.JobState, &item.Title, &item.State, &item.Attempts,
			&item.NextAttempt, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan transfer notification: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) RetryTransferNotificationByUser(
	ctx context.Context, userID int64, jobID, eventType string, now time.Time,
) (TransferNotification, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return TransferNotification{}, fmt.Errorf("begin notification retry: %w", err)
	}
	defer tx.Rollback()
	var item TransferNotification
	err = tx.QueryRowContext(ctx, `
		SELECT n.job_id, n.event_type, j.state, j.title, n.state, n.attempts, n.next_attempt_at, n.created_at, n.updated_at
		FROM transfer_notifications n
		JOIN transfer_jobs j ON j.id = n.job_id
		WHERE j.user_id = ? AND n.job_id = ? AND n.event_type = ?`, userID, jobID, eventType).Scan(
		&item.JobID, &item.EventType, &item.JobState, &item.Title, &item.State, &item.Attempts,
		&item.NextAttempt, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return TransferNotification{}, ErrNotificationNotFound
	}
	if err != nil {
		return TransferNotification{}, err
	}
	if item.State != "needs_attention" {
		return TransferNotification{}, ErrNotificationNotRetryable
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE transfer_notifications SET state = 'pending', next_attempt_at = 0, updated_at = ?
		WHERE job_id = ? AND event_type = ? AND state = 'needs_attention'`, now.UTC().Unix(), jobID, eventType)
	if err != nil {
		return TransferNotification{}, err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return TransferNotification{}, ErrNotificationNotRetryable
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO transfer_job_events (job_id, state, message, created_at)
		VALUES (?, ?, '用户确认重新发送企业微信通知（可能重复送达）', ?)`, jobID, item.JobState, now.UTC().Unix()); err != nil {
		return TransferNotification{}, err
	}
	if err := tx.Commit(); err != nil {
		return TransferNotification{}, err
	}
	item.State = "pending"
	item.NextAttempt = 0
	item.UpdatedAt = now.UTC().Unix()
	return item, nil
}
