package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadParsesProviderConfiguration(t *testing.T) {
	values := map[string]string{
		"MEDIA_HUB_ADDR":                     "127.0.0.1:9090",
		"MEDIA_HUB_DATABASE_PATH":            "/tmp/media-hub-test.db",
		"MEDIA_HUB_PROBE_TIMEOUT":            "750ms",
		"MEDIA_HUB_ENABLE_FIXTURES":          "true",
		"MEDIA_HUB_SECURE_COOKIES":           "true",
		"MEDIA_HUB_ADMIN_PASSWORD":           strings.Repeat("a", 16),
		"MEDIA_HUB_QMS_URL":                  "http://qms.local/",
		"MEDIA_HUB_QMS_API_KEY":              "qms-test-value",
		"MEDIA_HUB_EMBY_URL":                 "https://emby.local",
		"MEDIA_HUB_EMBY_API_KEY":             "emby-test-value",
		"MEDIA_HUB_115_ACCESS_TOKEN":         "115-test-value",
		"MEDIA_HUB_TMDB_ACCESS_TOKEN":        "tmdb-token",
		"MEDIA_HUB_SOURCE_FRAME_URL":         "https://frame.local/api",
		"MEDIA_HUB_DATA_ENCRYPTION_KEY":      "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=",
		"MEDIA_HUB_QMS_ACCOUNT_ID":           "7",
		"MEDIA_HUB_115_MOVIE_DESTINATION_ID": "100",
		"MEDIA_HUB_QMS_MOVIE_TARGET_PATH":    "/media/movies",
		"MEDIA_HUB_EMBY_MOVIE_LIBRARY_ID":    "library-movies",
		"MEDIA_HUB_SOURCE_FRAME_TOKEN":       "frame-test-value",
		"MEDIA_HUB_ANDROID_RELEASE_DIR":      "/srv/media-hub/releases",
	}

	loaded, err := load(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if loaded.Address != "127.0.0.1:9090" || loaded.DatabasePath != "/tmp/media-hub-test.db" {
		t.Fatal("unexpected server configuration")
	}
	if loaded.ProbeTimeout != 750*time.Millisecond || !loaded.FixtureMode || !loaded.SecureCookies {
		t.Fatal("unexpected runtime configuration")
	}
	if loaded.BootstrapAdminPassword == "" {
		t.Fatal("bootstrap administrator password was not loaded")
	}
	if loaded.QMediaSync.BaseURL != "http://qms.local" || loaded.QMediaSync.APIKey == "" {
		t.Fatal("unexpected QMediaSync configuration")
	}
	if loaded.Emby.BaseURL != "https://emby.local" || loaded.Emby.APIKey == "" {
		t.Fatal("unexpected Emby configuration")
	}
	if loaded.TMDB.BaseURL != "https://api.themoviedb.org/3" || loaded.TMDB.AccessToken == "" {
		t.Fatal("unexpected TMDB configuration")
	}
	if loaded.Sources[0].ID != "framehdr" || loaded.Sources[0].BaseURL != "https://frame.local/api" {
		t.Fatal("unexpected source configuration")
	}
	if loaded.DataEncryptionKey == "" || loaded.Workflow.QMediaSyncAccountID != 7 {
		t.Fatal("unexpected workflow configuration")
	}
	if loaded.AndroidReleaseDir != "/srv/media-hub/releases" {
		t.Fatal("unexpected Android release directory")
	}
	if target, ok := loaded.Workflow.Target("movie"); !ok || target.DestinationID != "100" {
		t.Fatal("unexpected movie workflow target")
	}
}

func TestLoadRejectsCredentialsInProviderURL(t *testing.T) {
	_, err := load(func(key string) (string, bool) {
		if key == "MEDIA_HUB_EMBY_URL" {
			return "http://user-value:password-value@emby.local", true
		}
		return "", false
	})
	if err == nil {
		t.Fatal("expected URL validation error")
	}
}

func TestLoadRejectsInvalidFixtureFlag(t *testing.T) {
	_, err := load(func(key string) (string, bool) {
		if key == "MEDIA_HUB_ENABLE_FIXTURES" {
			return "sometimes", true
		}
		return "", false
	})
	if err == nil {
		t.Fatal("expected fixture flag validation error")
	}
}

func TestLoadRejectsShortBootstrapPassword(t *testing.T) {
	_, err := load(func(key string) (string, bool) {
		if key == "MEDIA_HUB_ADMIN_PASSWORD" {
			return "too-short", true
		}
		return "", false
	})
	if err == nil {
		t.Fatal("expected administrator password validation error")
	}
}

func TestLoadRejectsPartialWeComConfiguration(t *testing.T) {
	_, err := load(func(key string) (string, bool) {
		if key == "MEDIA_HUB_WECOM_CORP_ID" {
			return "corp", true
		}
		return "", false
	})
	if err == nil {
		t.Fatal("expected partial WeCom configuration error")
	}
}

func TestLoadRejectsInvalidQMediaSyncAccountID(t *testing.T) {
	_, err := load(func(key string) (string, bool) {
		if key == "MEDIA_HUB_QMS_ACCOUNT_ID" {
			return "zero", true
		}
		return "", false
	})
	if err == nil {
		t.Fatal("expected QMediaSync account ID validation error")
	}
}

func TestLoadConfiguresEightCanonicalSources(t *testing.T) {
	values := map[string]string{
		"MEDIA_HUB_SOURCE_DIAN_URL":     "https://dian.local",
		"MEDIA_HUB_SOURCE_FRAMEHDR_URL": "https://framehdr.local",
		"MEDIA_HUB_SOURCE_GIMY_URL":     "https://gimy.local",
		"MEDIA_HUB_SOURCE_GUANYING_URL": "https://guanying.local",
		"MEDIA_HUB_SOURCE_HDHIVE_URL":   "https://hdhive.local",
		"MEDIA_HUB_SOURCE_JUYING_URL":   "https://juying.local",
		"MEDIA_HUB_SOURCE_MIKAN_URL":    "https://mikan.local",
		"MEDIA_HUB_SOURCE_SIDHUB_URL":   "https://sidhub.local",
	}
	loaded, err := load(func(key string) (string, bool) { value, ok := values[key]; return value, ok })
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Sources) != 8 || loaded.Sources[0].ID != "dian" || loaded.Sources[7].ID != "sidhub" {
		t.Fatalf("sources = %#v", loaded.Sources)
	}
}

func TestAndroidReleaseDirectoryRequiresAbsolutePath(t *testing.T) {
	if _, err := load(testLookup(map[string]string{"MEDIA_HUB_ANDROID_RELEASE_DIR": "relative/releases"})); err == nil {
		t.Fatal("expected relative Android release directory to fail")
	}
}

func TestLocalUploadRootsRequireAbsolutePaths(t *testing.T) {
	if _, err := load(testLookup(map[string]string{"MEDIA_HUB_LOCAL_UPLOAD_ROOTS": "relative/path"})); err == nil {
		t.Fatal("expected relative local upload root to fail")
	}
	configuration, err := load(testLookup(map[string]string{"MEDIA_HUB_LOCAL_UPLOAD_ROOTS": "/mnt/incoming,/mnt/staging,/mnt/incoming"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(configuration.LocalUploadRoots) != 2 {
		t.Fatalf("roots = %v", configuration.LocalUploadRoots)
	}
}

func testLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
