package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type AdminUser struct {
	ID           int64
	Username     string
	PasswordHash string
}

type Session struct {
	TokenHash  []byte
	UserID     int64
	ClientType string
	CSRFHash   []byte
	ExpiresAt  time.Time
	RevokedAt  *time.Time
}

func (s *Store) Admin(ctx context.Context) (AdminUser, bool, error) {
	var user AdminUser
	err := s.database.QueryRowContext(
		ctx,
		"SELECT id, username, password_hash FROM users WHERE username = ?",
		"admin",
	).Scan(&user.ID, &user.Username, &user.PasswordHash)
	if err == sql.ErrNoRows {
		return AdminUser{}, false, nil
	}
	if err != nil {
		return AdminUser{}, false, fmt.Errorf("read admin user: %w", err)
	}
	return user, true, nil
}

func (s *Store) EnsureAdmin(ctx context.Context, passwordHash string) (bool, error) {
	if passwordHash == "" {
		return false, fmt.Errorf("password hash is required")
	}
	result, err := s.database.ExecContext(ctx, `
		INSERT INTO users (username, password_hash)
		VALUES ('admin', ?)
		ON CONFLICT(username) DO NOTHING
	`, passwordHash)
	if err != nil {
		return false, fmt.Errorf("create admin user: %w", err)
	}
	created, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read admin creation result: %w", err)
	}
	return created == 1, nil
}

func (s *Store) CreateSession(ctx context.Context, session Session) error {
	if len(session.TokenHash) != 32 || session.UserID == 0 {
		return fmt.Errorf("valid session token hash and user ID are required")
	}
	if session.ClientType != "web" && session.ClientType != "android" {
		return fmt.Errorf("invalid session client type")
	}
	if session.ClientType == "web" && len(session.CSRFHash) != 32 {
		return fmt.Errorf("web session requires a CSRF hash")
	}
	if session.ClientType == "android" {
		session.CSRFHash = nil
	}

	_, err := s.database.ExecContext(ctx, `
		INSERT INTO sessions (token_hash, user_id, client_type, csrf_hash, expires_at)
		VALUES (?, ?, ?, ?, ?)
	`, session.TokenHash, session.UserID, session.ClientType, session.CSRFHash, session.ExpiresAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (s *Store) SessionByTokenHash(ctx context.Context, tokenHash []byte) (Session, bool, error) {
	if len(tokenHash) != 32 {
		return Session{}, false, nil
	}

	var session Session
	var expiresAt string
	var revokedAt sql.NullString
	err := s.database.QueryRowContext(ctx, `
		SELECT token_hash, user_id, client_type, csrf_hash, expires_at, revoked_at
		FROM sessions
		WHERE token_hash = ?
	`, tokenHash).Scan(
		&session.TokenHash,
		&session.UserID,
		&session.ClientType,
		&session.CSRFHash,
		&expiresAt,
		&revokedAt,
	)
	if err == sql.ErrNoRows {
		return Session{}, false, nil
	}
	if err != nil {
		return Session{}, false, fmt.Errorf("read session: %w", err)
	}

	session.ExpiresAt, err = time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return Session{}, false, fmt.Errorf("parse session expiry: %w", err)
	}
	if revokedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, revokedAt.String)
		if err != nil {
			return Session{}, false, fmt.Errorf("parse session revocation: %w", err)
		}
		session.RevokedAt = &parsed
	}
	return session, true, nil
}

func (s *Store) RevokeSession(ctx context.Context, tokenHash []byte, revokedAt time.Time) error {
	if len(tokenHash) != 32 {
		return nil
	}
	_, err := s.database.ExecContext(ctx, `
		UPDATE sessions
		SET revoked_at = ?
		WHERE token_hash = ? AND revoked_at IS NULL
	`, revokedAt.UTC().Format(time.RFC3339Nano), tokenHash)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (s *Store) PurgeExpiredSessions(ctx context.Context, now time.Time) error {
	_, err := s.database.ExecContext(
		ctx,
		"DELETE FROM sessions WHERE expires_at <= ? OR revoked_at IS NOT NULL",
		now.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("purge sessions: %w", err)
	}
	return nil
}

func (s *Store) UpdateAdminPassword(
	ctx context.Context, userID int64, passwordHash string, currentTokenHash []byte, revokedAt time.Time,
) error {
	if userID == 0 || passwordHash == "" || len(currentTokenHash) != 32 {
		return fmt.Errorf("valid user, password hash, and current session are required")
	}
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin password update: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ? AND username = 'admin'`, passwordHash, userID)
	if err != nil {
		return fmt.Errorf("update administrator password: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return fmt.Errorf("administrator user not found")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE sessions SET revoked_at = ?
		WHERE user_id = ? AND token_hash <> ? AND revoked_at IS NULL`,
		revokedAt.UTC().Format(time.RFC3339Nano), userID, currentTokenHash,
	); err != nil {
		return fmt.Errorf("revoke other sessions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit password update: %w", err)
	}
	return nil
}
