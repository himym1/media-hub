package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrSubXCommandNotFound     = errors.New("subx command not found")
	ErrSubXCommandNotRetryable = errors.New("subx command is not retryable")
)

type SubXCommandJob struct {
	ID             string
	UserID         int64
	OperationID    string
	IdempotencyKey string
	RequestHash    []byte
	PayloadToken   string
	ResultToken    string
	State          string
	Attempts       int
	ErrorCode      string
	ErrorMessage   string
	Retryable      bool
	CreatedAt      int64
	UpdatedAt      int64
}

type SubXCommandEvent struct {
	ID        int64
	State     string
	Message   string
	CreatedAt int64
}

const subXCommandColumns = `
	id, user_id, operation_id, idempotency_key, request_hash, payload_token,
	result_token, state, attempts, error_code, error_message, retryable, created_at, updated_at`

func (s *Store) CreateSubXCommand(ctx context.Context, job SubXCommandJob) (SubXCommandJob, bool, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return SubXCommandJob{}, false, fmt.Errorf("begin SubX command creation: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO subx_command_jobs (
			id, user_id, operation_id, idempotency_key, request_hash, payload_token,
			state, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, idempotency_key) DO NOTHING`,
		job.ID, job.UserID, job.OperationID, job.IdempotencyKey, job.RequestHash,
		job.PayloadToken, job.State, job.CreatedAt, job.UpdatedAt,
	)
	if err != nil {
		return SubXCommandJob{}, false, fmt.Errorf("insert SubX command: %w", err)
	}
	created, err := result.RowsAffected()
	if err != nil {
		return SubXCommandJob{}, false, err
	}
	if created == 0 {
		existing, err := subXCommandByIdempotency(ctx, tx, job.UserID, job.IdempotencyKey)
		if err != nil {
			return SubXCommandJob{}, false, err
		}
		if !bytes.Equal(existing.RequestHash, job.RequestHash) {
			return SubXCommandJob{}, false, ErrIdempotencyConflict
		}
		if err := tx.Commit(); err != nil {
			return SubXCommandJob{}, false, err
		}
		return existing, false, nil
	}
	if err := insertSubXCommandEvent(ctx, tx, job.ID, job.State, "命令已创建", job.CreatedAt); err != nil {
		return SubXCommandJob{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return SubXCommandJob{}, false, fmt.Errorf("commit SubX command creation: %w", err)
	}
	return job, true, nil
}

func (s *Store) SubXCommand(ctx context.Context, userID int64, id string) (SubXCommandJob, error) {
	job, err := scanSubXCommand(s.database.QueryRowContext(ctx, `SELECT `+subXCommandColumns+`
		FROM subx_command_jobs WHERE id = ? AND user_id = ?`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return SubXCommandJob{}, ErrSubXCommandNotFound
	}
	return job, err
}

func (s *Store) ListSubXCommands(ctx context.Context, userID int64, limit int) ([]SubXCommandJob, error) {
	rows, err := s.database.QueryContext(ctx, `SELECT `+subXCommandColumns+`
		FROM subx_command_jobs WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list SubX commands: %w", err)
	}
	defer rows.Close()
	jobs := make([]SubXCommandJob, 0)
	for rows.Next() {
		job, err := scanSubXCommand(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) SubXCommandEvents(ctx context.Context, userID int64, id string) ([]SubXCommandEvent, error) {
	if _, err := s.SubXCommand(ctx, userID, id); err != nil {
		return nil, err
	}
	rows, err := s.database.QueryContext(ctx, `
		SELECT id, state, message, created_at FROM subx_command_events
		WHERE job_id = ? ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]SubXCommandEvent, 0)
	for rows.Next() {
		var event SubXCommandEvent
		if err := rows.Scan(&event.ID, &event.State, &event.Message, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) NextQueuedSubXCommand(ctx context.Context) (SubXCommandJob, bool, error) {
	job, err := scanSubXCommand(s.database.QueryRowContext(ctx, `SELECT `+subXCommandColumns+`
		FROM subx_command_jobs WHERE state = 'queued' ORDER BY created_at LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return SubXCommandJob{}, false, nil
	}
	if err != nil {
		return SubXCommandJob{}, false, err
	}
	return job, true, nil
}

func (s *Store) BeginSubXCommand(ctx context.Context, id string, now time.Time) (SubXCommandJob, bool, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return SubXCommandJob{}, false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE subx_command_jobs SET state = 'submitting', attempts = attempts + 1,
			error_code = '', error_message = '', retryable = 0, updated_at = ?
		WHERE id = ? AND state = 'queued'`, now.UTC().Unix(), id)
	if err != nil {
		return SubXCommandJob{}, false, err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return SubXCommandJob{}, false, err
	}
	if err := insertSubXCommandEvent(ctx, tx, id, "submitting", "命令正在提交", now.UTC().Unix()); err != nil {
		return SubXCommandJob{}, false, err
	}
	job, err := scanSubXCommand(tx.QueryRowContext(ctx, `SELECT `+subXCommandColumns+` FROM subx_command_jobs WHERE id = ?`, id))
	if err != nil {
		return SubXCommandJob{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return SubXCommandJob{}, false, err
	}
	return job, true, nil
}

func (s *Store) FinishSubXCommand(ctx context.Context, job SubXCommandJob, expectedState, message string) error {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE subx_command_jobs SET result_token = ?, state = ?, error_code = ?,
			error_message = ?, retryable = ?, updated_at = ?
		WHERE id = ? AND state = ?`,
		job.ResultToken, job.State, job.ErrorCode, job.ErrorMessage, job.Retryable,
		job.UpdatedAt, job.ID, expectedState,
	)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrSubXCommandNotRetryable
	}
	if err := insertSubXCommandEvent(ctx, tx, job.ID, job.State, message, job.UpdatedAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RetrySubXCommand(ctx context.Context, userID int64, id string, now time.Time) (SubXCommandJob, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return SubXCommandJob{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE subx_command_jobs SET state = 'queued', result_token = '', error_code = '',
			error_message = '', retryable = 0, updated_at = ?
		WHERE id = ? AND user_id = ? AND state IN ('failed', 'needs_attention')`, now.UTC().Unix(), id, userID)
	if err != nil {
		return SubXCommandJob{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return SubXCommandJob{}, err
	}
	if count != 1 {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
			SELECT 1 FROM subx_command_jobs WHERE id = ? AND user_id = ?
		)`, id, userID).Scan(&exists); err != nil {
			return SubXCommandJob{}, err
		}
		if !exists {
			return SubXCommandJob{}, ErrSubXCommandNotFound
		}
		return SubXCommandJob{}, ErrSubXCommandNotRetryable
	}
	if err := insertSubXCommandEvent(ctx, tx, id, "queued", "用户已确认重新执行", now.UTC().Unix()); err != nil {
		return SubXCommandJob{}, err
	}
	job, err := scanSubXCommand(tx.QueryRowContext(ctx, `SELECT `+subXCommandColumns+` FROM subx_command_jobs WHERE id = ?`, id))
	if err != nil {
		return SubXCommandJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return SubXCommandJob{}, err
	}
	return job, nil
}

func (s *Store) MarkInterruptedSubXCommands(ctx context.Context, now time.Time) error {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id FROM subx_command_jobs WHERE state = 'submitting'`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `
			UPDATE subx_command_jobs SET state = 'needs_attention', error_code = 'submission_interrupted',
				error_message = '服务在命令提交期间停止，结果需要人工确认', retryable = 0, updated_at = ?
			WHERE id = ?`, now.UTC().Unix(), id); err != nil {
			return err
		}
		if err := insertSubXCommandEvent(ctx, tx, id, "needs_attention", "提交结果未知，未自动重放", now.UTC().Unix()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func subXCommandByIdempotency(ctx context.Context, tx *sql.Tx, userID int64, key string) (SubXCommandJob, error) {
	return scanSubXCommand(tx.QueryRowContext(ctx, `SELECT `+subXCommandColumns+`
		FROM subx_command_jobs WHERE user_id = ? AND idempotency_key = ?`, userID, key))
}

func insertSubXCommandEvent(ctx context.Context, tx *sql.Tx, id, state, message string, createdAt int64) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO subx_command_events (job_id, state, message, created_at) VALUES (?, ?, ?, ?)`, id, state, message, createdAt)
	return err
}

func scanSubXCommand(scanner subscriptionScanner) (SubXCommandJob, error) {
	var job SubXCommandJob
	var retryable int
	err := scanner.Scan(
		&job.ID, &job.UserID, &job.OperationID, &job.IdempotencyKey, &job.RequestHash,
		&job.PayloadToken, &job.ResultToken, &job.State, &job.Attempts, &job.ErrorCode,
		&job.ErrorMessage, &retryable, &job.CreatedAt, &job.UpdatedAt,
	)
	job.Retryable = retryable != 0
	return job, err
}

func (s *Store) CountBlockingSubXCommands(ctx context.Context, userID int64) (int, error) {
	var count int
	err := s.database.QueryRowContext(ctx, `SELECT count(*) FROM subx_command_jobs WHERE user_id = ? AND state IN ('queued','submitting','needs_attention')`, userID).Scan(&count)
	return count, err
}
