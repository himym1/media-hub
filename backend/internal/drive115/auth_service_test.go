package drive115

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"media-hub/backend/internal/securepayload"
	"media-hub/backend/internal/store"
)

func TestDeviceAuthorizationPersistsEncryptedCookieSession(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	if _, err := dataStore.EnsureAdmin(ctx, "password-hash"); err != nil {
		t.Fatal(err)
	}
	admin, _, err := dataStore.Admin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	codec, err := securepayload.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/token":
			_, _ = w.Write([]byte(`{"state":1,"data":{"uid":"uid","time":123,"qrcode":"qr-content","sign":"signature"}}`))
		case "/status":
			_, _ = w.Write([]byte(`{"state":1,"data":{"status":2}}`))
		case "/login":
			if request.FormValue("account") != "uid" || request.FormValue("app") != "qandroid" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"state":1,"data":{"cookie":{"UID":"uid-secret","CID":"cid-secret","SEID":"seid-secret","KID":"kid-secret"}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	drive := NewClient("", time.Second)
	service := NewAuthService(dataStore, codec, drive, time.Second)
	service.tokenURL = server.URL + "/token"
	service.statusURL = server.URL + "/status"
	service.loginURL = server.URL + "/login"
	service.now = func() time.Time { return time.Unix(1_720_000_000, 0).UTC() }
	challenge, err := service.Start(ctx, admin.ID)
	if err != nil || challenge.QRCode != "qr-content" {
		t.Fatalf("challenge=%#v err=%v", challenge, err)
	}
	confirmed, err := service.Poll(ctx, admin.ID, challenge.ID)
	if err != nil || confirmed.State != "confirmed" || !strings.Contains(drive.session(), "UID=uid-secret") {
		t.Fatalf("confirmed=%#v session=%q err=%v", confirmed, drive.session(), err)
	}
	sealed, exists, err := dataStore.ProviderCredential(ctx, admin.ID, "115")
	if err != nil || !exists || strings.Contains(sealed, "uid-secret") || strings.Contains(sealed, "seid-secret") {
		t.Fatalf("credential exists=%v sealed=%q err=%v", exists, sealed, err)
	}
}

func TestLoadAllowsMissingCookieSession(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	if _, err := dataStore.EnsureAdmin(ctx, "password-hash"); err != nil {
		t.Fatal(err)
	}
	admin, exists, err := dataStore.Admin(ctx)
	if err != nil || !exists {
		t.Fatalf("admin exists=%v err=%v", exists, err)
	}
	codec, err := securepayload.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	service := NewAuthService(dataStore, codec, NewClient("", time.Second), time.Second)
	if err := service.Load(ctx, admin.ID); err != nil {
		t.Fatalf("load missing session: %v", err)
	}
	if _, err := service.Status(ctx); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("status error = %v, want ErrNotConfigured", err)
	}
}

func TestQRCodeStateClassification(t *testing.T) {
	for _, raw := range []string{"1", "true"} {
		if !qrStateSuccessful(json.RawMessage(raw)) {
			t.Fatalf("state %s should be successful", raw)
		}
	}
	for _, raw := range []string{"0", "false"} {
		if !qrStateExpired(json.RawMessage(raw)) {
			t.Fatalf("state %s should be expired", raw)
		}
	}
	if qrStateSuccessful(nil) || qrStateExpired(nil) {
		t.Fatal("missing state must not be accepted")
	}
}
