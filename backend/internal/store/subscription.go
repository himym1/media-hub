package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrSubscriptionNotFound  = errors.New("subscription not found")
	ErrSubscriptionConflict  = errors.New("subscription already exists")
	ErrSubscriptionRunActive = errors.New("subscription already has an active run")
	ErrSubscriptionRunLost   = errors.New("subscription run state changed")
)

type Subscription struct {
	ID              string
	UserID          int64
	TMDBID          string
	Title           string
	OriginalTitle   string
	Year            int
	MediaType       string
	Season          int
	Policy          string
	Enabled         bool
	IntervalMinutes int
	SourceIDsJSON   string
	PreferencesJSON string
	NextRunAt       int64
	LastRunAt       int64
	LastEpisode     int
	CreatedAt       int64
	UpdatedAt       int64
}

type SubscriptionRun struct {
	ID                   string
	SubscriptionID       string
	TriggerType          string
	State                string
	ResumeState          string
	SourceID             string
	CandidateFingerprint string
	TransferJobID        string
	Attempts             int
	ErrorCode            string
	Message              string
	Retryable            bool
	NextAttemptAt        int64
	StartedAt            int64
	FinishedAt           int64
	UpdatedAt            int64
}

const subscriptionColumns = `
	id, user_id, tmdb_id, title, original_title, year, media_type, season,
	policy, enabled, interval_minutes, source_ids_json, preferences_json,
	next_run_at, last_run_at, last_episode, created_at, updated_at`

const subscriptionRunColumns = `
	id, subscription_id, trigger_type, state, resume_state, source_id,
	candidate_fingerprint, COALESCE(transfer_job_id, ''), attempts, error_code,
	message, retryable, next_attempt_at, started_at, finished_at, updated_at`

func (s *Store) CreateSubscription(ctx context.Context, item Subscription) (Subscription, error) {
	result, err := s.database.ExecContext(ctx, `
		INSERT INTO subscriptions (
			id, user_id, tmdb_id, title, original_title, year, media_type, season,
			policy, enabled, interval_minutes, source_ids_json, preferences_json,
			next_run_at, last_run_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, tmdb_id, media_type, season) DO NOTHING`,
		item.ID, item.UserID, item.TMDBID, item.Title, item.OriginalTitle, item.Year,
		item.MediaType, item.Season, item.Policy, item.Enabled, item.IntervalMinutes,
		item.SourceIDsJSON, item.PreferencesJSON, item.NextRunAt, item.LastRunAt,
		item.CreatedAt, item.UpdatedAt,
	)
	if err != nil {
		return Subscription{}, fmt.Errorf("insert subscription: %w", err)
	}
	created, err := result.RowsAffected()
	if err != nil {
		return Subscription{}, fmt.Errorf("read subscription creation result: %w", err)
	}
	if created == 0 {
		return Subscription{}, ErrSubscriptionConflict
	}
	return item, nil
}

func (s *Store) Subscription(ctx context.Context, userID int64, id string) (Subscription, error) {
	item, err := scanSubscription(s.database.QueryRowContext(ctx, `SELECT `+subscriptionColumns+`
		FROM subscriptions WHERE id = ? AND user_id = ?`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return Subscription{}, ErrSubscriptionNotFound
	}
	if err != nil {
		return Subscription{}, fmt.Errorf("read subscription: %w", err)
	}
	return item, nil
}

func (s *Store) ListSubscriptions(ctx context.Context, userID int64) ([]Subscription, error) {
	rows, err := s.database.QueryContext(ctx, `SELECT `+subscriptionColumns+`
		FROM subscriptions WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()
	items := make([]Subscription, 0)
	for rows.Next() {
		item, err := scanSubscription(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpdateSubscription(ctx context.Context, item Subscription) (Subscription, error) {
	result, err := s.database.ExecContext(ctx, `
		UPDATE subscriptions SET title = ?, original_title = ?, year = ?, policy = ?, enabled = ?,
			interval_minutes = ?, source_ids_json = ?, preferences_json = ?, next_run_at = ?, updated_at = ?
		WHERE id = ? AND user_id = ?`,
		item.Title, item.OriginalTitle, item.Year, item.Policy, item.Enabled,
		item.IntervalMinutes, item.SourceIDsJSON, item.PreferencesJSON, item.NextRunAt,
		item.UpdatedAt, item.ID, item.UserID,
	)
	if err != nil {
		return Subscription{}, fmt.Errorf("update subscription: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return Subscription{}, err
	}
	if count == 0 {
		return Subscription{}, ErrSubscriptionNotFound
	}
	return item, nil
}

func (s *Store) SetSubscriptionEnabled(ctx context.Context, userID int64, id string, enabled bool, nextRunAt int64, now time.Time) (Subscription, error) {
	result, err := s.database.ExecContext(ctx, `
		UPDATE subscriptions SET enabled = ?, next_run_at = ?, updated_at = ?
		WHERE id = ? AND user_id = ?`, enabled, nextRunAt, now.UTC().Unix(), id, userID)
	if err != nil {
		return Subscription{}, fmt.Errorf("toggle subscription: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return Subscription{}, err
	}
	if count == 0 {
		return Subscription{}, ErrSubscriptionNotFound
	}
	return s.Subscription(ctx, userID, id)
}

func (s *Store) DeleteSubscription(ctx context.Context, userID int64, id string) error {
	result, err := s.database.ExecContext(ctx, `DELETE FROM subscriptions WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrSubscriptionNotFound
	}
	return nil
}

func (s *Store) ClaimDueSubscription(ctx context.Context, runID string, now time.Time) (Subscription, SubscriptionRun, bool, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return Subscription{}, SubscriptionRun{}, false, fmt.Errorf("begin due subscription claim: %w", err)
	}
	defer tx.Rollback()
	item, err := scanSubscription(tx.QueryRowContext(ctx, `SELECT `+subscriptionColumns+`
		FROM subscriptions WHERE enabled = 1 AND next_run_at <= ? ORDER BY next_run_at, created_at LIMIT 1`, now.UTC().Unix()))
	if errors.Is(err, sql.ErrNoRows) {
		return Subscription{}, SubscriptionRun{}, false, nil
	}
	if err != nil {
		return Subscription{}, SubscriptionRun{}, false, fmt.Errorf("read due subscription: %w", err)
	}
	var active bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM subscription_runs
		WHERE subscription_id = ? AND state IN ('queued', 'searching', 'retry_wait', 'enqueued')
	)`, item.ID).Scan(&active); err != nil {
		return Subscription{}, SubscriptionRun{}, false, fmt.Errorf("check active subscription run: %w", err)
	}
	next := now.UTC().Add(time.Duration(item.IntervalMinutes) * time.Minute).Unix()
	result, err := tx.ExecContext(ctx, `
		UPDATE subscriptions SET next_run_at = ?, last_run_at = ?, updated_at = ?
		WHERE id = ? AND enabled = 1 AND next_run_at <= ?`,
		next, now.UTC().Unix(), now.UTC().Unix(), item.ID, now.UTC().Unix())
	if err != nil {
		return Subscription{}, SubscriptionRun{}, false, fmt.Errorf("claim due subscription: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return Subscription{}, SubscriptionRun{}, false, err
	}
	if count != 1 {
		return Subscription{}, SubscriptionRun{}, false, ErrSubscriptionRunLost
	}
	if active {
		if err := tx.Commit(); err != nil {
			return Subscription{}, SubscriptionRun{}, false, fmt.Errorf("commit deferred subscription claim: %w", err)
		}
		item.NextRunAt = next
		item.LastRunAt = now.UTC().Unix()
		item.UpdatedAt = now.UTC().Unix()
		return item, SubscriptionRun{}, true, nil
	}
	run := SubscriptionRun{
		ID: runID, SubscriptionID: item.ID, TriggerType: "scheduled", State: "queued",
		StartedAt: now.UTC().Unix(), UpdatedAt: now.UTC().Unix(),
	}
	if err := insertSubscriptionRun(ctx, tx, run); err != nil {
		return Subscription{}, SubscriptionRun{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Subscription{}, SubscriptionRun{}, false, fmt.Errorf("commit due subscription claim: %w", err)
	}
	item.NextRunAt = next
	item.LastRunAt = now.UTC().Unix()
	item.UpdatedAt = now.UTC().Unix()
	return item, run, true, nil
}

func (s *Store) CreateManualSubscriptionRun(ctx context.Context, userID int64, subscriptionID, runID string, now time.Time) (SubscriptionRun, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return SubscriptionRun{}, fmt.Errorf("begin manual subscription run: %w", err)
	}
	defer tx.Rollback()
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM subscriptions WHERE id = ? AND user_id = ?
	)`, subscriptionID, userID).Scan(&exists); err != nil {
		return SubscriptionRun{}, err
	}
	if !exists {
		return SubscriptionRun{}, ErrSubscriptionNotFound
	}
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM subscription_runs
		WHERE subscription_id = ? AND state IN ('queued', 'searching', 'retry_wait', 'enqueued')
	)`, subscriptionID).Scan(&exists); err != nil {
		return SubscriptionRun{}, err
	}
	if exists {
		return SubscriptionRun{}, ErrSubscriptionRunActive
	}
	run := SubscriptionRun{
		ID: runID, SubscriptionID: subscriptionID, TriggerType: "manual", State: "queued",
		StartedAt: now.UTC().Unix(), UpdatedAt: now.UTC().Unix(),
	}
	if err := insertSubscriptionRun(ctx, tx, run); err != nil {
		return SubscriptionRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return SubscriptionRun{}, fmt.Errorf("commit manual subscription run: %w", err)
	}
	return run, nil
}

func (s *Store) NextRunnableSubscriptionRun(ctx context.Context, now time.Time) (SubscriptionRun, bool, error) {
	run, err := scanSubscriptionRun(s.database.QueryRowContext(ctx, `SELECT `+subscriptionRunColumns+`
		FROM subscription_runs
		WHERE state IN ('queued', 'searching') OR (state = 'retry_wait' AND next_attempt_at <= ?)
		ORDER BY started_at LIMIT 1`, now.UTC().Unix()))
	if errors.Is(err, sql.ErrNoRows) {
		return SubscriptionRun{}, false, nil
	}
	if err != nil {
		return SubscriptionRun{}, false, fmt.Errorf("read runnable subscription run: %w", err)
	}
	return run, true, nil
}

func (s *Store) NextEnqueuedSubscriptionRun(ctx context.Context) (SubscriptionRun, bool, error) {
	run, err := scanSubscriptionRun(s.database.QueryRowContext(ctx, `SELECT `+subscriptionRunColumns+`
		FROM subscription_runs WHERE state = 'enqueued' ORDER BY updated_at LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return SubscriptionRun{}, false, nil
	}
	if err != nil {
		return SubscriptionRun{}, false, fmt.Errorf("read enqueued subscription run: %w", err)
	}
	return run, true, nil
}

func (s *Store) UpdateSubscriptionRun(ctx context.Context, run SubscriptionRun, expectedState string) error {
	result, err := s.database.ExecContext(ctx, `
		UPDATE subscription_runs SET state = ?, resume_state = ?, source_id = ?, candidate_fingerprint = ?,
			transfer_job_id = NULLIF(?, ''), attempts = ?, error_code = ?, message = ?, retryable = ?,
			next_attempt_at = ?, finished_at = ?, updated_at = ?
		WHERE id = ? AND state = ?`,
		run.State, run.ResumeState, run.SourceID, run.CandidateFingerprint, run.TransferJobID,
		run.Attempts, run.ErrorCode, run.Message, run.Retryable, run.NextAttemptAt,
		run.FinishedAt, run.UpdatedAt, run.ID, expectedState,
	)
	if err != nil {
		return fmt.Errorf("update subscription run: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrSubscriptionRunLost
	}
	return nil
}

func (s *Store) SubscriptionRuns(ctx context.Context, userID int64, subscriptionID string, limit int) ([]SubscriptionRun, error) {
	if _, err := s.Subscription(ctx, userID, subscriptionID); err != nil {
		return nil, err
	}
	rows, err := s.database.QueryContext(ctx, `SELECT `+subscriptionRunColumns+`
		FROM subscription_runs WHERE subscription_id = ? ORDER BY started_at DESC LIMIT ?`, subscriptionID, limit)
	if err != nil {
		return nil, fmt.Errorf("list subscription runs: %w", err)
	}
	defer rows.Close()
	runs := make([]SubscriptionRun, 0)
	for rows.Next() {
		run, err := scanSubscriptionRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *Store) SubscriptionByID(ctx context.Context, id string) (Subscription, error) {
	item, err := scanSubscription(s.database.QueryRowContext(ctx, `SELECT `+subscriptionColumns+`
		FROM subscriptions WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Subscription{}, ErrSubscriptionNotFound
	}
	if err != nil {
		return Subscription{}, fmt.Errorf("read subscription by ID: %w", err)
	}
	return item, nil
}

func (s *Store) HasTransferForIdentity(ctx context.Context, userID int64, mediaType, tmdbID string, season, episodeStart, episodeEnd int) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(
		SELECT 1 FROM transfer_jobs
		WHERE user_id = ? AND media_type = ? AND tmdb_id = ? AND season = ? AND state <> 'failed'`
	arguments := []any{userID, mediaType, tmdbID, season}
	if episodeStart > 0 {
		query += ` AND episode_start > 0 AND episode_start <= ? AND episode_end >= ?`
		arguments = append(arguments, episodeStart, episodeEnd)
	} else {
		query += ` AND episode_start = 0 AND episode_end = 0`
	}
	query += `)`
	err := s.database.QueryRowContext(ctx, query, arguments...).Scan(&exists)
	return exists, err
}

func (s *Store) HasSubscriptionCandidate(ctx context.Context, subscriptionID, fingerprint string) (bool, error) {
	var exists bool
	err := s.database.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM subscription_runs
		WHERE subscription_id = ? AND candidate_fingerprint = ?
		  AND state IN ('enqueued', 'completed', 'failed', 'needs_attention')
	)`, subscriptionID, fingerprint).Scan(&exists)
	return exists, err
}

func insertSubscriptionRun(ctx context.Context, tx *sql.Tx, run SubscriptionRun) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO subscription_runs (
			id, subscription_id, trigger_type, state, resume_state, source_id,
			candidate_fingerprint, transfer_job_id, attempts, error_code, message,
			retryable, next_attempt_at, started_at, finished_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.SubscriptionID, run.TriggerType, run.State, run.ResumeState, run.SourceID,
		run.CandidateFingerprint, run.TransferJobID, run.Attempts, run.ErrorCode, run.Message,
		run.Retryable, run.NextAttemptAt, run.StartedAt, run.FinishedAt, run.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert subscription run: %w", err)
	}
	return nil
}

type subscriptionScanner interface {
	Scan(...any) error
}

func scanSubscription(scanner subscriptionScanner) (Subscription, error) {
	var item Subscription
	var enabled int
	err := scanner.Scan(
		&item.ID, &item.UserID, &item.TMDBID, &item.Title, &item.OriginalTitle, &item.Year,
		&item.MediaType, &item.Season, &item.Policy, &enabled, &item.IntervalMinutes,
		&item.SourceIDsJSON, &item.PreferencesJSON, &item.NextRunAt, &item.LastRunAt, &item.LastEpisode,
		&item.CreatedAt, &item.UpdatedAt,
	)
	item.Enabled = enabled != 0
	return item, err
}

func scanSubscriptionRun(scanner subscriptionScanner) (SubscriptionRun, error) {
	var run SubscriptionRun
	var retryable int
	err := scanner.Scan(
		&run.ID, &run.SubscriptionID, &run.TriggerType, &run.State, &run.ResumeState,
		&run.SourceID, &run.CandidateFingerprint, &run.TransferJobID, &run.Attempts,
		&run.ErrorCode, &run.Message, &retryable, &run.NextAttemptAt, &run.StartedAt,
		&run.FinishedAt, &run.UpdatedAt,
	)
	run.Retryable = retryable != 0
	return run, err
}

func (s *Store) ImportSubscriptions(ctx context.Context, items []Subscription) (int, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin subscription import: %w", err)
	}
	defer tx.Rollback()
	created := 0
	for _, item := range items {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO subscriptions (
				id, user_id, tmdb_id, title, original_title, year, media_type, season,
				policy, enabled, interval_minutes, source_ids_json, preferences_json,
				next_run_at, last_run_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(user_id, tmdb_id, media_type, season) DO NOTHING`,
			item.ID, item.UserID, item.TMDBID, item.Title, item.OriginalTitle, item.Year,
			item.MediaType, item.Season, item.Policy, item.Enabled, item.IntervalMinutes,
			item.SourceIDsJSON, item.PreferencesJSON, item.NextRunAt, item.LastRunAt,
			item.CreatedAt, item.UpdatedAt,
		)
		if err != nil {
			return 0, fmt.Errorf("import subscription: %w", err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		created += int(count)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit subscription import: %w", err)
	}
	return created, nil
}

func (s *Store) SetSubscriptionsEnabled(ctx context.Context, userID int64, ids []string, enabled bool, nextRunAt int64, now time.Time) error {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin subscription batch update: %w", err)
	}
	defer tx.Rollback()
	for _, id := range ids {
		result, err := tx.ExecContext(ctx, `
			UPDATE subscriptions SET enabled = ?, next_run_at = ?, updated_at = ?
			WHERE id = ? AND user_id = ?`, enabled, nextRunAt, now.UTC().Unix(), id, userID)
		if err != nil {
			return fmt.Errorf("update subscription batch: %w", err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count != 1 {
			return ErrSubscriptionNotFound
		}
	}
	return tx.Commit()
}

func (s *Store) AdvanceSubscriptionEpisode(ctx context.Context, id string, episode int, now time.Time) error {
	if episode <= 0 {
		return nil
	}
	_, err := s.database.ExecContext(ctx, `
		UPDATE subscriptions SET last_episode = CASE WHEN last_episode < ? THEN ? ELSE last_episode END, updated_at = ?
		WHERE id = ?`, episode, episode, now.UTC().Unix(), id)
	return err
}
