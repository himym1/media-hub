package desktoprelease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServiceReadsAndVerifiesRelease(t *testing.T) {
	directory := t.TempDir()
	installer := []byte("nsis setup fixture")
	writeReleaseFixture(t, directory, PlatformWindows, 20038, installer)

	service := NewService(directory)
	release, err := service.Latest(context.Background(), PlatformWindows)
	if err != nil {
		t.Fatal(err)
	}
	if release.VersionCode != 20038 || release.DownloadPath != "/api/v1/client/desktop/releases/20038/installer" {
		t.Fatalf("unexpected release: %#v", release)
	}

	_, file, err := service.OpenInstaller(context.Background(), 20038, PlatformWindows)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != string(installer) {
		t.Fatal("unexpected installer content")
	}
}

func TestServiceReadsDarwinRelease(t *testing.T) {
	directory := t.TempDir()
	dmg := []byte("apple disk image")
	writeReleaseFixture(t, directory, PlatformDarwin, 20038, dmg)

	service := NewService(directory)
	release, err := service.Latest(context.Background(), PlatformDarwin)
	if err != nil {
		t.Fatal(err)
	}
	if release.DownloadPath != "/api/v1/client/desktop/releases/20038/dmg" {
		t.Fatalf("download path = %q", release.DownloadPath)
	}
	if _, err := service.Latest(context.Background(), PlatformWindows); !errors.Is(err, ErrNotFound) {
		t.Fatalf("windows lookup error = %v, want ErrNotFound", err)
	}
}

func TestServiceRejectsTamperedInstaller(t *testing.T) {
	directory := t.TempDir()
	writeReleaseFixture(t, directory, PlatformWindows, 20038, []byte("original setup"))
	if err := os.WriteFile(filepath.Join(directory, "media-hub-20038.exe"), []byte("tampered setup"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err := NewService(directory).OpenInstaller(context.Background(), 20038, PlatformWindows)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("error = %v, want ErrInvalid", err)
	}
}

func TestServiceRequiresConfiguredDirectory(t *testing.T) {
	_, err := NewService("").Latest(context.Background(), PlatformWindows)
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("error = %v, want ErrNotConfigured", err)
	}
}

func TestParsePlatform(t *testing.T) {
	windows, err := ParsePlatform("")
	if err != nil || windows != PlatformWindows {
		t.Fatalf("default platform = %q err=%v", windows, err)
	}
	darwin, err := ParsePlatform("macOS")
	if err != nil || darwin != PlatformDarwin {
		t.Fatalf("darwin platform = %q err=%v", darwin, err)
	}
	if _, err := ParsePlatform("linux"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("linux error = %v, want ErrInvalid", err)
	}
}

func writeReleaseFixture(t *testing.T, directory string, platform Platform, versionCode int, installer []byte) {
	t.Helper()
	hash := sha256.Sum256(installer)
	value := manifest{
		VersionCode:             versionCode,
		VersionName:             "0.20.38",
		MinimumSupportedVersion: 1,
		SHA256:                  hex.EncodeToString(hash[:]),
		SizeBytes:               int64(len(installer)),
		PublishedAt:             time.Date(2026, 9, 14, 8, 0, 0, 0, time.UTC),
		Notes:                   "First private desktop release",
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, manifestName(platform)), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("media-hub-%d.%s", versionCode, artifactExt(platform))), installer, 0o600); err != nil {
		t.Fatal(err)
	}
}
