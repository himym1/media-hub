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
	ErrIdempotencyConflict   = errors.New("idempotency key was already used for another request")
	ErrTransferNotFound      = errors.New("transfer job not found")
	ErrTransferNotRetryable  = errors.New("transfer job is not retryable")
	ErrTransferNotArchivable = errors.New("transfer job is not archivable")
	ErrTransferNotDeletable  = errors.New("transfer job is not deletable")
)

type TransferJob struct {
	ID             string
	UserID         int64
	IdempotencyKey string
	RequestHash    []byte
	SelectionToken string
	SourceID       string
	CandidateID    string
	Title          string
	Year           int
	Season         int
	EpisodeStart   int
	EpisodeEnd     int
	MediaType      string
	TMDBID         string
	State          string
	ResumeState    string
	ProviderToken  string
	EmbyItemID     string
	Attempts       int
	ErrorCode      string
	ErrorMessage   string
	Retryable      bool
	NextAttemptAt  int64
	CreatedAt      int64
	UpdatedAt      int64
	ArchivedAt     int64
}

type TransferEvent struct {
	ID        int64
	State     string
	Message   string
	CreatedAt int64
}

const transferColumns = `
	id, user_id, idempotency_key, request_hash, selection_token,
	source_id, candidate_id, title, year, season, episode_start, episode_end, media_type, tmdb_id, state, resume_state,
	provider_token, emby_item_id,
	attempts, error_code, error_message, retryable, next_attempt_at,
	created_at, updated_at, archived_at`

func (s *Store) CreateTransferJob(ctx context.Context, job TransferJob) (TransferJob, bool, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return TransferJob{}, false, fmt.Errorf("begin transfer creation: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO transfer_jobs (
			id, user_id, idempotency_key, request_hash, selection_token,
			source_id, candidate_id, title, year, season, episode_start, episode_end, media_type, tmdb_id, state,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, idempotency_key) DO NOTHING`,
		job.ID, job.UserID, job.IdempotencyKey, job.RequestHash, job.SelectionToken,
		job.SourceID, job.CandidateID, job.Title, job.Year, job.Season, job.EpisodeStart, job.EpisodeEnd,
		job.MediaType, job.TMDBID, job.State, job.CreatedAt, job.UpdatedAt,
	)
	if err != nil {
		return TransferJob{}, false, fmt.Errorf("insert transfer job: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return TransferJob{}, false, fmt.Errorf("read transfer creation result: %w", err)
	}
	if rows == 0 {
		existing, err := transferByIdempotency(ctx, tx, job.UserID, job.IdempotencyKey)
		if err != nil {
			return TransferJob{}, false, err
		}
		if !bytes.Equal(existing.RequestHash, job.RequestHash) {
			return TransferJob{}, false, ErrIdempotencyConflict
		}
		if err := tx.Commit(); err != nil {
			return TransferJob{}, false, fmt.Errorf("commit existing transfer lookup: %w", err)
		}
		return existing, false, nil
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO transfer_job_events (job_id, state, message, created_at)
		VALUES (?, ?, ?, ?)`, job.ID, job.State, "任务已创建", job.CreatedAt); err != nil {
		return TransferJob{}, false, fmt.Errorf("insert transfer event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return TransferJob{}, false, fmt.Errorf("commit transfer creation: %w", err)
	}
	return job, true, nil
}

func (s *Store) TransferJob(ctx context.Context, userID int64, jobID string) (TransferJob, error) {
	row := s.database.QueryRowContext(ctx, `SELECT `+transferColumns+`
		FROM transfer_jobs WHERE id = ? AND user_id = ?`, jobID, userID)
	job, err := scanTransfer(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TransferJob{}, ErrTransferNotFound
	}
	return job, err
}

func (s *Store) ListTransferJobs(ctx context.Context, userID int64, limit int, archived bool) ([]TransferJob, error) {
	archiveFilter := "archived_at = 0"
	if archived {
		archiveFilter = "archived_at > 0"
	}
	rows, err := s.database.QueryContext(ctx, `SELECT `+transferColumns+`
		FROM transfer_jobs WHERE user_id = ? AND `+archiveFilter+` ORDER BY created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list transfer jobs: %w", err)
	}
	defer rows.Close()
	jobs := make([]TransferJob, 0)
	for rows.Next() {
		job, err := scanTransfer(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transfer jobs: %w", err)
	}
	return jobs, nil
}

func (s *Store) SetTransferArchived(ctx context.Context, userID int64, jobID string, archived bool, now time.Time) (TransferJob, error) {
	job, err := s.TransferJob(ctx, userID, jobID)
	if err != nil {
		return TransferJob{}, err
	}
	if archived == (job.ArchivedAt > 0) {
		return job, nil
	}
	if archived && job.State != "completed" {
		return TransferJob{}, ErrTransferNotArchivable
	}

	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return TransferJob{}, fmt.Errorf("begin transfer archive update: %w", err)
	}
	defer tx.Rollback()

	timestamp := now.UTC().Unix()
	archivedAt := int64(0)
	message := "任务已恢复到当前列表"
	query := `UPDATE transfer_jobs SET archived_at = 0 WHERE id = ? AND user_id = ? AND archived_at > 0`
	arguments := []any{jobID, userID}
	if archived {
		archivedAt = timestamp
		message = "任务已归档"
		query = `UPDATE transfer_jobs SET archived_at = ?
			WHERE id = ? AND user_id = ? AND archived_at = 0 AND state = 'completed'`
		arguments = []any{archivedAt, jobID, userID}
	}
	result, err := tx.ExecContext(ctx, query, arguments...)
	if err != nil {
		return TransferJob{}, fmt.Errorf("update transfer archive: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return TransferJob{}, fmt.Errorf("read transfer archive result: %w", err)
	}
	if count == 0 {
		return TransferJob{}, ErrTransferNotArchivable
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO transfer_job_events (job_id, state, message, created_at)
		VALUES (?, ?, ?, ?)`, job.ID, job.State, message, timestamp); err != nil {
		return TransferJob{}, fmt.Errorf("record transfer archive event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return TransferJob{}, fmt.Errorf("commit transfer archive update: %w", err)
	}
	job.ArchivedAt = archivedAt
	return job, nil
}

func (s *Store) DeleteTransferJob(ctx context.Context, userID int64, jobID string) error {
	job, err := s.TransferJob(ctx, userID, jobID)
	if err != nil {
		return err
	}
	if job.State != "failed" && job.State != "needs_attention" {
		return ErrTransferNotDeletable
	}
	result, err := s.database.ExecContext(ctx, `
		DELETE FROM transfer_jobs
		WHERE id = ? AND user_id = ? AND state IN ('failed', 'needs_attention')`,
		jobID, userID)
	if err != nil {
		return fmt.Errorf("delete transfer job: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read transfer delete result: %w", err)
	}
	if count == 0 {
		return ErrTransferNotDeletable
	}
	return nil
}

func (s *Store) TransferEvents(ctx context.Context, userID int64, jobID string) ([]TransferEvent, error) {
	if _, err := s.TransferJob(ctx, userID, jobID); err != nil {
		return nil, err
	}
	rows, err := s.database.QueryContext(ctx, `
		SELECT id, state, message, created_at
		FROM transfer_job_events WHERE job_id = ? ORDER BY id`, jobID)
	if err != nil {
		return nil, fmt.Errorf("list transfer events: %w", err)
	}
	defer rows.Close()
	events := make([]TransferEvent, 0)
	for rows.Next() {
		var event TransferEvent
		if err := rows.Scan(&event.ID, &event.State, &event.Message, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan transfer event: %w", err)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) NextRunnableTransfer(ctx context.Context, now time.Time) (TransferJob, bool, error) {
	row := s.database.QueryRowContext(ctx, `SELECT `+transferColumns+`
		FROM transfer_jobs
		WHERE state IN ('queued', 'transferred')
		   OR (state IN (
			   'transferring', 'retry_wait', 'syncing', 'refreshing_emby',
			   'indexing_emby', 'verifying_playback'
		   ) AND next_attempt_at <= ?)
		ORDER BY created_at LIMIT 1`, now.UTC().Unix())
	job, err := scanTransfer(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TransferJob{}, false, nil
	}
	return job, err == nil, err
}

func (s *Store) UpdateTransferJob(ctx context.Context, job TransferJob, expectedState, eventMessage string) (bool, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin transfer update: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE transfer_jobs SET
			state = ?, resume_state = ?, provider_token = ?, emby_item_id = ?, attempts = ?, error_code = ?,
			error_message = ?, retryable = ?, next_attempt_at = ?, updated_at = ?
		WHERE id = ? AND state = ?`,
		job.State, job.ResumeState, job.ProviderToken, job.EmbyItemID, job.Attempts, job.ErrorCode,
		job.ErrorMessage, job.Retryable, job.NextAttemptAt, job.UpdatedAt,
		job.ID, expectedState,
	)
	if err != nil {
		return false, fmt.Errorf("update transfer job: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read transfer update result: %w", err)
	}
	if count == 0 {
		return false, nil
	}
	if job.State != expectedState || eventMessage != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO transfer_job_events (job_id, state, message, created_at)
			VALUES (?, ?, ?, ?)`, job.ID, job.State, eventMessage, job.UpdatedAt); err != nil {
			return false, fmt.Errorf("insert transfer event: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit transfer update: %w", err)
	}
	return true, nil
}

func (s *Store) RetryTransferJob(ctx context.Context, userID int64, jobID string, now time.Time) (TransferJob, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return TransferJob{}, fmt.Errorf("begin transfer retry: %w", err)
	}
	defer tx.Rollback()
	row := tx.QueryRowContext(ctx, `SELECT `+transferColumns+`
		FROM transfer_jobs WHERE id = ? AND user_id = ?`, jobID, userID)
	job, err := scanTransfer(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TransferJob{}, ErrTransferNotFound
	}
	if err != nil {
		return TransferJob{}, err
	}
	recoverSourceTransfer := job.State == "failed" && job.ErrorCode == "provider_state_invalid" && job.ProviderToken == "" && job.SelectionToken != ""
	if job.State != "needs_attention" && (job.State != "failed" || (!job.Retryable && !recoverSourceTransfer)) {
		return TransferJob{}, ErrTransferNotRetryable
	}
	resumeState := job.ResumeState
	if recoverSourceTransfer {
		resumeState = "transferring"
	}
	if resumeState == "" && job.State == "needs_attention" {
		resumeState = "transferred"
	}
	if resumeState == "" {
		resumeState = "queued"
	}
	job.State = resumeState
	job.ResumeState = ""
	job.ErrorCode = ""
	job.ErrorMessage = ""
	job.Retryable = false
	job.Attempts = 0
	job.NextAttemptAt = 0
	job.UpdatedAt = now.UTC().Unix()
	if _, err := tx.ExecContext(ctx, `
		UPDATE transfer_jobs SET state = ?, resume_state = '', error_code = '',
			error_message = '', retryable = 0, attempts = 0, next_attempt_at = 0, updated_at = ?
		WHERE id = ?`, job.State, job.UpdatedAt, job.ID); err != nil {
		return TransferJob{}, fmt.Errorf("retry transfer job: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO transfer_job_events (job_id, state, message, created_at)
		VALUES (?, ?, ?, ?)`, job.ID, job.State, "用户请求重试", job.UpdatedAt); err != nil {
		return TransferJob{}, fmt.Errorf("insert transfer retry event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return TransferJob{}, fmt.Errorf("commit transfer retry: %w", err)
	}
	return job, nil
}

func (s *Store) MarkInterruptedSyncSubmissions(ctx context.Context, now time.Time) error {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin interrupted submission recovery: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id FROM transfer_jobs WHERE state = 'submitting_sync'`)
	if err != nil {
		return fmt.Errorf("list interrupted submissions: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan interrupted submission: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close interrupted submissions: %w", err)
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `
			UPDATE transfer_jobs SET state = 'needs_attention', resume_state = 'transferred',
				error_code = 'sync_submission_unknown', error_message = '同步提交结果未知',
				retryable = 1, updated_at = ? WHERE id = ?`, now.UTC().Unix(), id); err != nil {
			return fmt.Errorf("recover interrupted submission: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO transfer_job_events (job_id, state, message, created_at)
			VALUES (?, 'needs_attention', '服务重启时同步提交结果未知', ?)`, id, now.UTC().Unix()); err != nil {
			return fmt.Errorf("record interrupted submission: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit interrupted submission recovery: %w", err)
	}
	return nil
}

func transferByIdempotency(ctx context.Context, tx *sql.Tx, userID int64, key string) (TransferJob, error) {
	job, err := scanTransfer(tx.QueryRowContext(ctx, `SELECT `+transferColumns+`
		FROM transfer_jobs WHERE user_id = ? AND idempotency_key = ?`, userID, key))
	if err != nil {
		return TransferJob{}, fmt.Errorf("read idempotent transfer: %w", err)
	}
	return job, nil
}

type transferScanner interface {
	Scan(...any) error
}

func (s *Store) TransferJobSourceID(ctx context.Context, jobID string) (string, error) {
	var sourceID string
	err := s.database.QueryRowContext(ctx, `SELECT source_id FROM transfer_jobs WHERE id = ?`, jobID).Scan(&sourceID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrTransferNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read transfer source: %w", err)
	}
	return sourceID, nil
}

func scanTransfer(scanner transferScanner) (TransferJob, error) {
	var job TransferJob
	var retryable int
	if err := scanner.Scan(
		&job.ID, &job.UserID, &job.IdempotencyKey, &job.RequestHash, &job.SelectionToken,
		&job.SourceID, &job.CandidateID, &job.Title, &job.Year, &job.Season, &job.EpisodeStart, &job.EpisodeEnd,
		&job.MediaType, &job.TMDBID, &job.State, &job.ResumeState, &job.ProviderToken, &job.EmbyItemID,
		&job.Attempts, &job.ErrorCode, &job.ErrorMessage, &retryable, &job.NextAttemptAt,
		&job.CreatedAt, &job.UpdatedAt, &job.ArchivedAt,
	); err != nil {
		return TransferJob{}, err
	}
	job.Retryable = retryable != 0
	return job, nil
}
