package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrLocalUploadNotFound = errors.New("local upload not found")

type LocalUploadJob struct {
	ID, IdempotencyKey, PayloadToken, State, ErrorCode, ErrorMessage string
	UserID, BytesDone, BytesTotal, CreatedAt, UpdatedAt              int64
	RequestHash                                                      []byte
}
type LocalUploadEvent struct {
	ID        int64  `json:"id"`
	State     string `json:"state"`
	Message   string `json:"message"`
	CreatedAt int64  `json:"createdAt"`
}

const localUploadColumns = `id,user_id,idempotency_key,request_hash,payload_token,state,bytes_done,bytes_total,error_code,error_message,created_at,updated_at`

func (s *Store) CreateLocalUpload(ctx context.Context, job LocalUploadJob) (LocalUploadJob, bool, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return LocalUploadJob{}, false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO local_upload_jobs(id,user_id,idempotency_key,request_hash,payload_token,state,bytes_total,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(user_id,idempotency_key) DO NOTHING`, job.ID, job.UserID, job.IdempotencyKey, job.RequestHash, job.PayloadToken, job.State, job.BytesTotal, job.CreatedAt, job.UpdatedAt)
	if err != nil {
		return LocalUploadJob{}, false, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return LocalUploadJob{}, false, err
	}
	if count == 0 {
		existing, err := scanLocalUpload(tx.QueryRowContext(ctx, `SELECT `+localUploadColumns+` FROM local_upload_jobs WHERE user_id=? AND idempotency_key=?`, job.UserID, job.IdempotencyKey))
		if err != nil {
			return LocalUploadJob{}, false, err
		}
		if !bytes.Equal(existing.RequestHash, job.RequestHash) {
			return LocalUploadJob{}, false, ErrIdempotencyConflict
		}
		if err := tx.Commit(); err != nil {
			return LocalUploadJob{}, false, err
		}
		return existing, false, nil
	}
	if err := insertLocalUploadEvent(ctx, tx, job.ID, job.State, "上传任务已创建", job.CreatedAt); err != nil {
		return LocalUploadJob{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return LocalUploadJob{}, false, err
	}
	return job, true, nil
}
func (s *Store) LocalUpload(ctx context.Context, userID int64, id string) (LocalUploadJob, error) {
	job, err := scanLocalUpload(s.database.QueryRowContext(ctx, `SELECT `+localUploadColumns+` FROM local_upload_jobs WHERE id=? AND user_id=?`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return LocalUploadJob{}, ErrLocalUploadNotFound
	}
	return job, err
}
func (s *Store) ListLocalUploads(ctx context.Context, userID int64, limit int) ([]LocalUploadJob, error) {
	rows, err := s.database.QueryContext(ctx, `SELECT `+localUploadColumns+` FROM local_upload_jobs WHERE user_id=? ORDER BY created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []LocalUploadJob{}
	for rows.Next() {
		value, err := scanLocalUpload(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
func (s *Store) LocalUploadEvents(ctx context.Context, userID int64, id string) ([]LocalUploadEvent, error) {
	if _, err := s.LocalUpload(ctx, userID, id); err != nil {
		return nil, err
	}
	rows, err := s.database.QueryContext(ctx, `SELECT id,state,message,created_at FROM local_upload_events WHERE job_id=? ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []LocalUploadEvent{}
	for rows.Next() {
		var value LocalUploadEvent
		if err := rows.Scan(&value.ID, &value.State, &value.Message, &value.CreatedAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
func (s *Store) NextLocalUpload(ctx context.Context) (LocalUploadJob, bool, error) {
	job, err := scanLocalUpload(s.database.QueryRowContext(ctx, `SELECT `+localUploadColumns+` FROM local_upload_jobs WHERE state='queued' ORDER BY created_at,id LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return LocalUploadJob{}, false, nil
	}
	return job, err == nil, err
}
func (s *Store) TransitionLocalUpload(ctx context.Context, id, from, to, message string, now time.Time) (LocalUploadJob, bool, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return LocalUploadJob{}, false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE local_upload_jobs SET state=?,error_code='',error_message='',updated_at=? WHERE id=? AND state=?`, to, now.UTC().Unix(), id, from)
	if err != nil {
		return LocalUploadJob{}, false, err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return LocalUploadJob{}, false, err
	}
	if err := insertLocalUploadEvent(ctx, tx, id, to, message, now.UTC().Unix()); err != nil {
		return LocalUploadJob{}, false, err
	}
	job, err := scanLocalUpload(tx.QueryRowContext(ctx, `SELECT `+localUploadColumns+` FROM local_upload_jobs WHERE id=?`, id))
	if err != nil {
		return LocalUploadJob{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return LocalUploadJob{}, false, err
	}
	return job, true, nil
}
func (s *Store) UpdateLocalUploadProgress(ctx context.Context, id string, done, total int64, now time.Time) error {
	_, err := s.database.ExecContext(ctx, `UPDATE local_upload_jobs SET bytes_done=?,bytes_total=?,updated_at=? WHERE id=? AND state IN ('hashing','submitting_init','uploading')`, done, total, now.UTC().Unix(), id)
	return err
}
func (s *Store) FinishLocalUpload(ctx context.Context, job LocalUploadJob, message string) error {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE local_upload_jobs SET state=?,bytes_done=?,bytes_total=?,error_code=?,error_message=?,updated_at=? WHERE id=? AND state IN ('hashing','submitting_init','uploading')`, job.State, job.BytesDone, job.BytesTotal, job.ErrorCode, job.ErrorMessage, job.UpdatedAt, job.ID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("local upload state changed")
	}
	if err := insertLocalUploadEvent(ctx, tx, job.ID, job.State, message, job.UpdatedAt); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) RetryLocalUpload(ctx context.Context, userID int64, id, confirmation string, now time.Time) (LocalUploadJob, error) {
	if id != confirmation {
		return LocalUploadJob{}, errors.New("confirmation mismatch")
	}
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return LocalUploadJob{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE local_upload_jobs SET state='queued',bytes_done=0,error_code='',error_message='',updated_at=? WHERE id=? AND user_id=? AND state IN ('failed','needs_attention')`, now.UTC().Unix(), id, userID)
	if err != nil {
		return LocalUploadJob{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return LocalUploadJob{}, err
	}
	if count != 1 {
		return LocalUploadJob{}, ErrLocalUploadNotFound
	}
	if err := insertLocalUploadEvent(ctx, tx, id, "queued", "用户确认后重新上传", now.UTC().Unix()); err != nil {
		return LocalUploadJob{}, err
	}
	job, err := scanLocalUpload(tx.QueryRowContext(ctx, `SELECT `+localUploadColumns+` FROM local_upload_jobs WHERE id=?`, id))
	if err != nil {
		return LocalUploadJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return LocalUploadJob{}, err
	}
	return job, nil
}
func (s *Store) MarkInterruptedLocalUploads(ctx context.Context, now time.Time) error {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id FROM local_upload_jobs WHERE state IN ('hashing','submitting_init','uploading')`)
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
		if _, err := tx.ExecContext(ctx, `UPDATE local_upload_jobs SET state='needs_attention',error_code='upload_interrupted',error_message='上传结果需要人工确认',updated_at=? WHERE id=?`, now.UTC().Unix(), id); err != nil {
			return err
		}
		if err := insertLocalUploadEvent(ctx, tx, id, "needs_attention", "上传中断，未自动重放", now.UTC().Unix()); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func insertLocalUploadEvent(ctx context.Context, tx *sql.Tx, id, state, message string, created int64) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO local_upload_events(job_id,state,message,created_at) VALUES(?,?,?,?)`, id, state, message, created)
	return err
}
func scanLocalUpload(scanner subscriptionScanner) (LocalUploadJob, error) {
	var value LocalUploadJob
	err := scanner.Scan(&value.ID, &value.UserID, &value.IdempotencyKey, &value.RequestHash, &value.PayloadToken, &value.State, &value.BytesDone, &value.BytesTotal, &value.ErrorCode, &value.ErrorMessage, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}
