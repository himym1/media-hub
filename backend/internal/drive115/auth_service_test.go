package drive115

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"media-hub/backend/internal/securepayload"
	"media-hub/backend/internal/store"
)

func TestDeviceAuthorizationPersistsEncryptedCredential(t *testing.T) {
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
		case "/device":
			if request.FormValue("client_id") != "client-id" || request.FormValue("code_challenge") == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"state":true,"data":{"uid":"uid","time":123,"qrcode":"qr-content","sign":"signature"}}`))
		case "/status":
			_, _ = w.Write([]byte(`{"state":1,"data":{"status":2}}`))
		case "/token":
			if request.FormValue("uid") != "uid" || request.FormValue("code_verifier") == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"access-secret","refresh_token":"refresh-secret","expires_in":3600}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	drive := NewClient("", time.Second)
	service := NewAuthService(dataStore, codec, "client-id", drive, time.Second)
	service.deviceCodeEndpoint = server.URL + "/device"
	service.deviceStatusURL = server.URL + "/status"
	service.deviceTokenURL = server.URL + "/token"
	service.now = func() time.Time { return time.Unix(1_720_000_000, 0).UTC() }
	challenge, err := service.Start(ctx, admin.ID)
	if err != nil || challenge.QRCode != "qr-content" {
		t.Fatalf("challenge=%#v err=%v", challenge, err)
	}
	confirmed, err := service.Poll(ctx, admin.ID, challenge.ID)
	if err != nil || confirmed.State != "confirmed" || drive.token() != "access-secret" {
		t.Fatalf("confirmed=%#v err=%v", confirmed, err)
	}
	sealed, exists, err := dataStore.ProviderCredential(ctx, admin.ID, "115")
	if err != nil || !exists || strings.Contains(sealed, "access-secret") || strings.Contains(sealed, "refresh-secret") {
		t.Fatalf("credential exists=%v sealed=%q err=%v", exists, sealed, err)
	}
}
