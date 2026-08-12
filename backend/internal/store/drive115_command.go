package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrDrive115CommandNotFound = errors.New("115 command not found")

type Drive115CommandJob struct {
	ID, Operation, IdempotencyKey, PayloadToken, ResultToken, State, ErrorCode, ErrorMessage string
	UserID                                                                                   int64
	RequestHash                                                                              []byte
	Attempts                                                                                 int
	CreatedAt, UpdatedAt                                                                     int64
}

type Drive115CommandEvent struct {
	ID        int64  `json:"id"`
	State     string `json:"state"`
	Message   string `json:"message"`
	CreatedAt int64  `json:"createdAt"`
}

const drive115CommandColumns = `id, user_id, operation, idempotency_key, request_hash, payload_token,
	result_token, state, attempts, error_code, error_message, created_at, updated_at`

func (s *Store) CreateDrive115Command(ctx context.Context, job Drive115CommandJob) (Drive115CommandJob, bool, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return Drive115CommandJob{}, false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO drive115_command_jobs
		(id,user_id,operation,idempotency_key,request_hash,payload_token,state,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(user_id,idempotency_key) DO NOTHING`,
		job.ID, job.UserID, job.Operation, job.IdempotencyKey, job.RequestHash, job.PayloadToken, job.State, job.CreatedAt, job.UpdatedAt)
	if err != nil {
		return Drive115CommandJob{}, false, fmt.Errorf("insert 115 command: %w", err)
	}
	created, err := result.RowsAffected()
	if err != nil {
		return Drive115CommandJob{}, false, err
	}
	if created == 0 {
		existing, err := scanDrive115Command(tx.QueryRowContext(ctx, `SELECT `+drive115CommandColumns+` FROM drive115_command_jobs WHERE user_id=? AND idempotency_key=?`, job.UserID, job.IdempotencyKey))
		if err != nil {
			return Drive115CommandJob{}, false, err
		}
		if !bytes.Equal(existing.RequestHash, job.RequestHash) {
			return Drive115CommandJob{}, false, ErrIdempotencyConflict
		}
		if err := tx.Commit(); err != nil {
			return Drive115CommandJob{}, false, err
		}
		return existing, false, nil
	}
	if err := insertDrive115CommandEvent(ctx, tx, job.ID, job.State, "115 命令已创建", job.CreatedAt); err != nil {
		return Drive115CommandJob{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Drive115CommandJob{}, false, err
	}
	return job, true, nil
}

func (s *Store) Drive115Command(ctx context.Context, userID int64, id string) (Drive115CommandJob, error) {
	job, err := scanDrive115Command(s.database.QueryRowContext(ctx, `SELECT `+drive115CommandColumns+` FROM drive115_command_jobs WHERE id=? AND user_id=?`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return Drive115CommandJob{}, ErrDrive115CommandNotFound
	}
	return job, err
}

func (s *Store) ListDrive115Commands(ctx context.Context, userID int64, limit int) ([]Drive115CommandJob, error) {
	rows, err := s.database.QueryContext(ctx, `SELECT `+drive115CommandColumns+` FROM drive115_command_jobs WHERE user_id=? ORDER BY created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]Drive115CommandJob, 0)
	for rows.Next() {
		job, err := scanDrive115Command(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) Drive115CommandEvents(ctx context.Context, userID int64, id string) ([]Drive115CommandEvent, error) {
	if _, err := s.Drive115Command(ctx, userID, id); err != nil {
		return nil, err
	}
	rows, err := s.database.QueryContext(ctx, `SELECT id,state,message,created_at FROM drive115_command_events WHERE job_id=? ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]Drive115CommandEvent, 0)
	for rows.Next() {
		var value Drive115CommandEvent
		if err := rows.Scan(&value.ID, &value.State, &value.Message, &value.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, value)
	}
	return events, rows.Err()
}

func (s *Store) ConfirmDrive115Command(ctx context.Context, userID int64, id, confirmation string, now time.Time) (Drive115CommandJob, error) {
	if confirmation != id {
		return Drive115CommandJob{}, errors.New("confirmation does not match command id")
	}
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return Drive115CommandJob{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE drive115_command_jobs SET state='queued',updated_at=? WHERE id=? AND user_id=? AND state='awaiting_confirmation'`, now.UTC().Unix(), id, userID)
	if err != nil {
		return Drive115CommandJob{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return Drive115CommandJob{}, err
	}
	if count != 1 {
		return Drive115CommandJob{}, ErrDrive115CommandNotFound
	}
	if err := insertDrive115CommandEvent(ctx, tx, id, "queued", "删除命令已明确确认", now.UTC().Unix()); err != nil {
		return Drive115CommandJob{}, err
	}
	job, err := scanDrive115Command(tx.QueryRowContext(ctx, `SELECT `+drive115CommandColumns+` FROM drive115_command_jobs WHERE id=?`, id))
	if err != nil {
		return Drive115CommandJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return Drive115CommandJob{}, err
	}
	return job, nil
}

func (s *Store) RetryDrive115Command(ctx context.Context, userID int64, id, confirmation string, now time.Time) (Drive115CommandJob, error) {
	if confirmation != id {
		return Drive115CommandJob{}, errors.New("confirmation does not match command id")
	}
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return Drive115CommandJob{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE drive115_command_jobs SET state='queued',result_token='',error_code='',error_message='',updated_at=? WHERE id=? AND user_id=? AND state IN ('failed','needs_attention')`, now.UTC().Unix(), id, userID)
	if err != nil {
		return Drive115CommandJob{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return Drive115CommandJob{}, err
	}
	if count != 1 {
		return Drive115CommandJob{}, ErrDrive115CommandNotFound
	}
	if err := insertDrive115CommandEvent(ctx, tx, id, "queued", "用户确认后重新执行", now.UTC().Unix()); err != nil {
		return Drive115CommandJob{}, err
	}
	job, err := scanDrive115Command(tx.QueryRowContext(ctx, `SELECT `+drive115CommandColumns+` FROM drive115_command_jobs WHERE id=?`, id))
	if err != nil {
		return Drive115CommandJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return Drive115CommandJob{}, err
	}
	return job, nil
}

func (s *Store) NextDrive115Command(ctx context.Context) (Drive115CommandJob, bool, error) {
	job, err := scanDrive115Command(s.database.QueryRowContext(ctx, `SELECT `+drive115CommandColumns+` FROM drive115_command_jobs WHERE state='queued' ORDER BY created_at LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return Drive115CommandJob{}, false, nil
	}
	return job, err == nil, err
}

func (s *Store) BeginDrive115Command(ctx context.Context, id string, now time.Time) (Drive115CommandJob, bool, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return Drive115CommandJob{}, false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE drive115_command_jobs SET state='submitting',attempts=attempts+1,error_code='',error_message='',updated_at=? WHERE id=? AND state='queued'`, now.UTC().Unix(), id)
	if err != nil {
		return Drive115CommandJob{}, false, err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return Drive115CommandJob{}, false, err
	}
	if err := insertDrive115CommandEvent(ctx, tx, id, "submitting", "115 命令正在提交", now.UTC().Unix()); err != nil {
		return Drive115CommandJob{}, false, err
	}
	job, err := scanDrive115Command(tx.QueryRowContext(ctx, `SELECT `+drive115CommandColumns+` FROM drive115_command_jobs WHERE id=?`, id))
	if err != nil {
		return Drive115CommandJob{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Drive115CommandJob{}, false, err
	}
	return job, true, nil
}

func (s *Store) FinishDrive115Command(ctx context.Context, job Drive115CommandJob, message string) error {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE drive115_command_jobs SET result_token=?,state=?,error_code=?,error_message=?,updated_at=? WHERE id=? AND state='submitting'`, job.ResultToken, job.State, job.ErrorCode, job.ErrorMessage, job.UpdatedAt, job.ID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("115 command state changed")
	}
	if err := insertDrive115CommandEvent(ctx, tx, job.ID, job.State, message, job.UpdatedAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) MarkInterruptedDrive115Commands(ctx context.Context, now time.Time) error {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id FROM drive115_command_jobs WHERE state='submitting'`)
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
		if _, err := tx.ExecContext(ctx, `UPDATE drive115_command_jobs SET state='needs_attention',error_code='submission_interrupted',error_message='服务在提交期间停止，结果需要人工确认',updated_at=? WHERE id=?`, now.UTC().Unix(), id); err != nil {
			return err
		}
		if err := insertDrive115CommandEvent(ctx, tx, id, "needs_attention", "提交结果未知，未自动重放", now.UTC().Unix()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func insertDrive115CommandEvent(ctx context.Context, tx *sql.Tx, id, state, message string, createdAt int64) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO drive115_command_events(job_id,state,message,created_at) VALUES(?,?,?,?)`, id, state, message, createdAt)
	return err
}

func scanDrive115Command(scanner subscriptionScanner) (Drive115CommandJob, error) {
	var job Drive115CommandJob
	err := scanner.Scan(&job.ID, &job.UserID, &job.Operation, &job.IdempotencyKey, &job.RequestHash, &job.PayloadToken, &job.ResultToken, &job.State, &job.Attempts, &job.ErrorCode, &job.ErrorMessage, &job.CreatedAt, &job.UpdatedAt)
	return job, err
}
