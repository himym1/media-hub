package auth

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"media-hub/backend/internal/store"
)

func TestServiceBootstrapsAuthenticatesAndRevokesSessions(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer dataStore.Close()

	service, err := NewService(dataStore)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	password := strings.Repeat("a", 16)
	created, err := service.Bootstrap(ctx, password)
	if err != nil || !created {
		t.Fatalf("bootstrap: created=%v err=%v", created, err)
	}

	session, err := service.Login(ctx, password, "web")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	principal, err := service.Authenticate(ctx, session.Token)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if err := service.ValidateCSRF(principal, session.CSRFToken); err != nil {
		t.Fatalf("validate CSRF: %v", err)
	}
	if err := service.Logout(ctx, principal); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := service.Authenticate(ctx, session.Token); err != ErrInvalidSession {
		t.Fatalf("authenticate revoked session error = %v", err)
	}
}

func TestChangePasswordRevokesOtherSessions(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	service, err := NewService(dataStore)
	if err != nil {
		t.Fatal(err)
	}
	current := strings.Repeat("a", 16)
	replacement := strings.Repeat("b", 16)
	if created, err := service.Bootstrap(ctx, current); err != nil || !created {
		t.Fatalf("bootstrap=%v err=%v", created, err)
	}
	web, err := service.Login(ctx, current, "web")
	if err != nil {
		t.Fatal(err)
	}
	android, err := service.Login(ctx, current, "android")
	if err != nil {
		t.Fatal(err)
	}
	principal, err := service.Authenticate(ctx, web.Token)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ChangePassword(ctx, principal, current, replacement); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, web.Token); err != nil {
		t.Fatalf("current session: %v", err)
	}
	if _, err := service.Authenticate(ctx, android.Token); err != ErrInvalidSession {
		t.Fatalf("other session error=%v", err)
	}
	if _, err := service.Login(ctx, current, "web"); err != ErrInvalidCredentials {
		t.Fatalf("old password error=%v", err)
	}
	if _, err := service.Login(ctx, replacement, "web"); err != nil {
		t.Fatalf("new password: %v", err)
	}
}
