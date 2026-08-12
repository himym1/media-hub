package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrActiveTransfers = errors.New("provider settings cannot change while recoverable transfer jobs exist")

var ErrProviderChallengeNotFound = errors.New("provider authorization challenge not found")

type ProviderAuthChallenge struct {
	ID           string
	UserID       int64
	Provider     string
	PayloadToken string
	State        string
	ExpiresAt    int64
	CreatedAt    int64
	UpdatedAt    int64
}

func (s *Store) UpsertProviderCredential(ctx context.Context, userID int64, provider, payloadToken string, now time.Time) error {
	if userID == 0 || provider == "" || payloadToken == "" {
		return fmt.Errorf("provider credential fields are required")
	}
	_, err := s.database.ExecContext(ctx, `
		INSERT INTO provider_credentials (user_id, provider, payload_token, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, provider) DO UPDATE SET payload_token = excluded.payload_token, updated_at = excluded.updated_at`,
		userID, provider, payloadToken, now.UTC().Unix())
	if err != nil {
		return fmt.Errorf("store provider credential: %w", err)
	}
	return nil
}

func (s *Store) UpsertProviderCredentialWithoutActiveTransfers(ctx context.Context, userID int64, provider, payloadToken string, now time.Time) error {
	if userID == 0 || provider == "" || payloadToken == "" {
		return fmt.Errorf("provider credential fields are required")
	}
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin provider settings update: %w", err)
	}
	defer tx.Rollback()
	var active bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM transfer_jobs WHERE user_id = ?
		AND (state NOT IN ('completed','failed') OR (state = 'failed' AND retryable = 1))
	)`, userID).Scan(&active); err != nil {
		return fmt.Errorf("check active transfer jobs: %w", err)
	}
	if active {
		return ErrActiveTransfers
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO provider_credentials (user_id, provider, payload_token, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, provider) DO UPDATE SET payload_token = excluded.payload_token, updated_at = excluded.updated_at`,
		userID, provider, payloadToken, now.UTC().Unix()); err != nil {
		return fmt.Errorf("store provider settings: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit provider settings update: %w", err)
	}
	return nil
}

func (s *Store) ProviderCredential(ctx context.Context, userID int64, provider string) (string, bool, error) {
	var token string
	err := s.database.QueryRowContext(ctx, `
		SELECT payload_token FROM provider_credentials WHERE user_id = ? AND provider = ?`,
		userID, provider).Scan(&token)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read provider credential: %w", err)
	}
	return token, true, nil
}

func (s *Store) CreateProviderAuthChallenge(ctx context.Context, value ProviderAuthChallenge) error {
	if value.ID == "" || value.UserID == 0 || value.Provider == "" || value.PayloadToken == "" || value.State != "pending" {
		return fmt.Errorf("valid provider authorization challenge is required")
	}
	_, err := s.database.ExecContext(ctx, `
		INSERT INTO provider_auth_challenges (id, user_id, provider, payload_token, state, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID, value.UserID, value.Provider, value.PayloadToken, value.State,
		value.ExpiresAt, value.CreatedAt, value.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create provider authorization challenge: %w", err)
	}
	return nil
}

func (s *Store) ProviderAuthChallenge(ctx context.Context, userID int64, id, provider string) (ProviderAuthChallenge, error) {
	var value ProviderAuthChallenge
	err := s.database.QueryRowContext(ctx, `
		SELECT id, user_id, provider, payload_token, state, expires_at, created_at, updated_at
		FROM provider_auth_challenges WHERE id = ? AND user_id = ? AND provider = ?`,
		id, userID, provider).Scan(
		&value.ID, &value.UserID, &value.Provider, &value.PayloadToken, &value.State,
		&value.ExpiresAt, &value.CreatedAt, &value.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ProviderAuthChallenge{}, ErrProviderChallengeNotFound
	}
	if err != nil {
		return ProviderAuthChallenge{}, fmt.Errorf("read provider authorization challenge: %w", err)
	}
	return value, nil
}

func (s *Store) SetProviderAuthChallengeState(ctx context.Context, userID int64, id, provider, fromState, toState string, now time.Time) error {
	result, err := s.database.ExecContext(ctx, `
		UPDATE provider_auth_challenges SET state = ?, updated_at = ?
		WHERE id = ? AND user_id = ? AND provider = ? AND state = ?`,
		toState, now.UTC().Unix(), id, userID, provider, fromState)
	if err != nil {
		return fmt.Errorf("update provider authorization challenge: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrProviderChallengeNotFound
	}
	return nil
}
