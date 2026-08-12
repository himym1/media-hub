package androidrelease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServiceReadsAndVerifiesRelease(t *testing.T) {
	directory := t.TempDir()
	apk := []byte("signed apk fixture")
	writeReleaseFixture(t, directory, 6000, apk)

	service := NewService(directory)
	release, err := service.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if release.VersionCode != 6000 || release.DownloadPath != "/api/v1/client/android/releases/6000/apk" {
		t.Fatalf("unexpected release: %#v", release)
	}

	_, file, err := service.OpenAPK(context.Background(), 6000)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != string(apk) {
		t.Fatal("unexpected APK content")
	}
}

func TestServiceRejectsTamperedAPK(t *testing.T) {
	directory := t.TempDir()
	writeReleaseFixture(t, directory, 6000, []byte("original apk"))
	if err := os.WriteFile(filepath.Join(directory, "media-hub-6000.apk"), []byte("tampered apk"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err := NewService(directory).OpenAPK(context.Background(), 6000)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("error = %v, want ErrInvalid", err)
	}
}

func TestServiceRequiresConfiguredDirectory(t *testing.T) {
	_, err := NewService("").Latest(context.Background())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("error = %v, want ErrNotConfigured", err)
	}
}

func writeReleaseFixture(t *testing.T, directory string, versionCode int, apk []byte) {
	t.Helper()
	hash := sha256.Sum256(apk)
	value := manifest{
		VersionCode:             versionCode,
		VersionName:             "0.6.0",
		MinimumSupportedVersion: 1,
		SHA256:                  hex.EncodeToString(hash[:]),
		SizeBytes:               int64(len(apk)),
		PublishedAt:             time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC),
		Notes:                   "First private release",
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "latest.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "media-hub-6000.apk"), apk, 0o600); err != nil {
		t.Fatal(err)
	}
}
