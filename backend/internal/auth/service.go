package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"media-hub/backend/internal/store"
)

var (
	ErrNotConfigured      = errors.New("administrator is not configured")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidSession     = errors.New("invalid session")
	ErrInvalidCSRF        = errors.New("invalid CSRF token")
	ErrInvalidPassword    = errors.New("invalid new password")
)

const sessionDuration = 30 * 24 * time.Hour

type Service struct {
	store     *store.Store
	now       func() time.Time
	dummyHash string
}

type SessionToken struct {
	Token     string
	CSRFToken string
	ExpiresAt time.Time
	Client    string
}

type Principal struct {
	UserID    int64
	Client    string
	TokenHash []byte
	CSRFHash  []byte
	ExpiresAt time.Time
}

func NewService(dataStore *store.Store) (*Service, error) {
	dummyHash, err := hashPassword("media-hub-dummy-password")
	if err != nil {
		return nil, err
	}
	return &Service{store: dataStore, now: time.Now, dummyHash: dummyHash}, nil
}

func (s *Service) Bootstrap(ctx context.Context, password string) (bool, error) {
	_, exists, err := s.store.Admin(ctx)
	if err != nil {
		return false, err
	}
	if exists || password == "" {
		return false, nil
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return false, err
	}
	return s.store.EnsureAdmin(ctx, passwordHash)
}

func (s *Service) Configured(ctx context.Context) (bool, error) {
	_, exists, err := s.store.Admin(ctx)
	return exists, err
}

func (s *Service) Login(ctx context.Context, password, client string) (SessionToken, error) {
	if client != "web" && client != "android" {
		return SessionToken{}, ErrInvalidCredentials
	}
	admin, exists, err := s.store.Admin(ctx)
	if err != nil {
		return SessionToken{}, err
	}
	if !exists {
		_ = verifyPassword(password, s.dummyHash)
		return SessionToken{}, ErrNotConfigured
	}
	if !verifyPassword(password, admin.PasswordHash) {
		return SessionToken{}, ErrInvalidCredentials
	}

	token, tokenHash, err := generateToken()
	if err != nil {
		return SessionToken{}, err
	}
	var csrfToken string
	var csrfHash []byte
	if client == "web" {
		csrfToken, csrfHash, err = generateToken()
		if err != nil {
			return SessionToken{}, err
		}
	}
	expiresAt := s.now().UTC().Add(sessionDuration)
	if err := s.store.CreateSession(ctx, store.Session{
		TokenHash:  tokenHash,
		UserID:     admin.ID,
		ClientType: client,
		CSRFHash:   csrfHash,
		ExpiresAt:  expiresAt,
	}); err != nil {
		return SessionToken{}, err
	}
	return SessionToken{
		Token: token, CSRFToken: csrfToken,
		ExpiresAt: expiresAt, Client: client,
	}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (Principal, error) {
	tokenHash, ok := decodeAndHashToken(token)
	if !ok {
		return Principal{}, ErrInvalidSession
	}
	session, exists, err := s.store.SessionByTokenHash(ctx, tokenHash)
	if err != nil {
		return Principal{}, err
	}
	if !exists || session.RevokedAt != nil || !s.now().UTC().Before(session.ExpiresAt) {
		return Principal{}, ErrInvalidSession
	}
	return Principal{
		UserID: session.UserID, Client: session.ClientType,
		TokenHash: session.TokenHash, CSRFHash: session.CSRFHash,
		ExpiresAt: session.ExpiresAt,
	}, nil
}

func (s *Service) ValidateCSRF(principal Principal, token string) error {
	if principal.Client != "web" {
		return nil
	}
	tokenHash, ok := decodeAndHashToken(token)
	if !ok || len(principal.CSRFHash) != len(tokenHash) || subtle.ConstantTimeCompare(principal.CSRFHash, tokenHash) != 1 {
		return ErrInvalidCSRF
	}
	return nil
}

func (s *Service) Logout(ctx context.Context, principal Principal) error {
	return s.store.RevokeSession(ctx, principal.TokenHash, s.now().UTC())
}

func (s *Service) ChangePassword(ctx context.Context, principal Principal, currentPassword, newPassword string) error {
	if len(newPassword) < 12 || len(newPassword) > 1024 || newPassword == currentPassword {
		return ErrInvalidPassword
	}
	admin, exists, err := s.store.Admin(ctx)
	if err != nil {
		return err
	}
	if !exists || admin.ID != principal.UserID || !verifyPassword(currentPassword, admin.PasswordHash) {
		return ErrInvalidCredentials
	}
	passwordHash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.store.UpdateAdminPassword(ctx, principal.UserID, passwordHash, principal.TokenHash, s.now().UTC())
}

func (s *Service) PurgeExpired(ctx context.Context) error {
	return s.store.PurgeExpiredSessions(ctx, s.now().UTC())
}

func generateToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate session token: %w", err)
	}
	hash := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(raw), hash[:], nil
}

func decodeAndHashToken(token string) ([]byte, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 {
		return nil, false
	}
	hash := sha256.Sum256(raw)
	return hash[:], true
}
