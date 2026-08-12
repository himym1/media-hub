package store

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestAdminAndSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	created, err := store.EnsureAdmin(ctx, "encoded-password-hash")
	if err != nil || !created {
		t.Fatalf("ensure admin: created=%v err=%v", created, err)
	}
	admin, found, err := store.Admin(ctx)
	if err != nil || !found || admin.Username != "admin" {
		t.Fatalf("read admin: found=%v err=%v", found, err)
	}

	tokenHash := bytes.Repeat([]byte{1}, 32)
	csrfHash := bytes.Repeat([]byte{2}, 32)
	expiresAt := time.Now().UTC().Add(time.Hour)
	if err := store.CreateSession(ctx, Session{
		TokenHash: tokenHash, UserID: admin.ID, ClientType: "web",
		CSRFHash: csrfHash, ExpiresAt: expiresAt,
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	session, found, err := store.SessionByTokenHash(ctx, tokenHash)
	if err != nil || !found || session.ClientType != "web" || !bytes.Equal(session.CSRFHash, csrfHash) {
		t.Fatalf("read session: found=%v err=%v", found, err)
	}

	revokedAt := time.Now().UTC()
	if err := store.RevokeSession(ctx, tokenHash, revokedAt); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	session, found, err = store.SessionByTokenHash(ctx, tokenHash)
	if err != nil || !found || session.RevokedAt == nil {
		t.Fatalf("read revoked session: found=%v err=%v", found, err)
	}
}
