package settings

import (
	"context"
	"encoding/base64"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"media-hub/backend/internal/config"
	"media-hub/backend/internal/securepayload"
	"media-hub/backend/internal/store"
)

func TestUpdatePreservesSecretsAndPersistsEncryptedSettings(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	if _, err := dataStore.EnsureAdmin(ctx, "test-hash"); err != nil {
		t.Fatal(err)
	}
	codec, err := securepayload.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}

	initial := Values{
		QMediaSync: config.QMediaSync{BaseURL: "https://qms.example", APIKey: "old-key"},
		TMDB:       config.TMDB{AccessToken: "tmdb-token"},
		SubX:       config.SubX{BaseURL: "https://subx.example", Username: "admin", Password: "subx-password", SourceEnabled: true},
		Sources:    []config.SearchSource{{ID: "framehdr", Label: "帧影", BaseURL: "https://source.example", Token: "source-token"}},
	}
	var applied Values
	service := NewService(dataStore, codec, initial, func(value Values) { applied = value })
	view, err := service.Update(ctx, 1, Update{
		QMediaSync: QMediaSyncUpdate{BaseURL: "https://qms-new.example"},
		TMDB:       TMDBUpdate{AccessToken: SecretUpdate{}},
		SubX:       &SubXUpdate{BaseURL: "https://subx.example", Username: "admin", SourceEnabled: true},
		Sources:    sourceUpdates("framehdr", "https://source-new.example"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !view.QMediaSync.APIKey.Configured || !view.TMDB.AccessToken.Configured || !view.SubX.Password.Configured || !view.Sources[1].Token.Configured {
		t.Fatalf("secret configuration was not preserved: %+v", view)
	}
	if applied.QMediaSync.APIKey != "old-key" || applied.TMDB.BaseURL != "https://api.themoviedb.org/3" || applied.SubX.Password != "subx-password" || applied.Sources[1].Token != "source-token" {
		t.Fatalf("applied settings = %+v", applied)
	}
	sealed, exists, err := dataStore.ProviderCredential(ctx, 1, ProviderKey)
	if err != nil || !exists {
		t.Fatalf("credential exists=%v err=%v", exists, err)
	}
	if sealed == "" || sealed == "old-key" || sealed == "source-token" {
		t.Fatal("settings were not encrypted")
	}

	reloaded := NewService(dataStore, codec, Values{}, nil)
	if err := reloaded.Load(ctx, 1); err != nil {
		t.Fatal(err)
	}
	values := reloaded.Values()
	if values.QMediaSync.APIKey != "old-key" || values.SubX.Password != "subx-password" || values.Sources[1].Token != "source-token" {
		t.Fatalf("reloaded values = %+v", values)
	}
}

func TestUpdateRejectsInvalidSourceAndSupportsExplicitSecretClear(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	if _, err := dataStore.EnsureAdmin(ctx, "test-hash"); err != nil {
		t.Fatal(err)
	}
	codec, _ := securepayload.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	service := NewService(dataStore, codec, Values{QMediaSync: config.QMediaSync{APIKey: "secret"}}, nil)

	invalidSources := sourceUpdates("framehdr", "")
	invalidSources[0].ID = "unknown"
	_, err = service.Update(ctx, 1, Update{Sources: invalidSources})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v", err)
	}

	view, err := service.Update(ctx, 1, Update{
		QMediaSync: QMediaSyncUpdate{APIKey: SecretUpdate{Clear: true}},
		Sources:    sourceUpdates("framehdr", ""),
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.QMediaSync.APIKey.Configured {
		t.Fatal("API key was not cleared")
	}
}

func TestMergePreservesSubXForOlderClients(t *testing.T) {
	current := Values{SubX: config.SubX{BaseURL: "https://subx.example", Username: "admin", Password: "secret", SourceEnabled: true}}
	merged := merge(current, Update{Sources: sourceUpdates("", "")})
	if merged.SubX != current.SubX {
		t.Fatalf("SubX settings changed without an explicit update: %+v", merged.SubX)
	}
}

func TestLoadLegacyPayloadPreservesSubXBaseline(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	if _, err := dataStore.EnsureAdmin(ctx, "test-hash"); err != nil {
		t.Fatal(err)
	}
	codec, _ := securepayload.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	legacy := struct {
		QMediaSync config.QMediaSync     `json:"qmediaSync"`
		Emby       config.Emby           `json:"emby"`
		Drive115   config.Drive115       `json:"drive115"`
		TMDB       config.TMDB           `json:"tmdb"`
		Workflow   config.Workflow       `json:"workflow"`
		Sources    []config.SearchSource `json:"sources"`
	}{Sources: configSources()}
	sealed, err := codec.Seal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := dataStore.UpsertProviderCredential(ctx, 1, ProviderKey, sealed, time.Now()); err != nil {
		t.Fatal(err)
	}
	baseline := config.SubX{BaseURL: "https://subx.example", Username: "admin", Password: "secret", SourceEnabled: true}
	service := NewService(dataStore, codec, Values{SubX: baseline, Sources: configSources()}, nil)
	if err := service.Load(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if service.Values().SubX != baseline {
		t.Fatalf("legacy load changed SubX baseline: %+v", service.Values().SubX)
	}
}

func sourceUpdates(configuredID, baseURL string) []SourceUpdate {
	ids := []string{"dian", "framehdr", "gimy", "guanying", "hdhive", "juying", "mikan", "sidhub"}
	result := make([]SourceUpdate, 0, len(ids))
	for _, id := range ids {
		source := SourceUpdate{ID: id}
		if id == configuredID {
			source.BaseURL = baseURL
		}
		result = append(result, source)
	}
	return result
}

func TestUpdateRejectsRecoverableTransferJobs(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	if _, err := dataStore.EnsureAdmin(ctx, "test-hash"); err != nil {
		t.Fatal(err)
	}
	admin, _, err := dataStore.Admin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	codec, _ := securepayload.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	applied := Values{}
	service := NewService(dataStore, codec, Values{}, func(value Values) { applied = value })
	job := store.TransferJob{
		ID: "active-job", UserID: admin.ID, IdempotencyKey: "active_job", RequestHash: []byte("request"),
		SelectionToken: "encrypted", SourceID: "framehdr", CandidateID: "candidate", Title: "Movie",
		MediaType: "movie", TMDBID: "1", State: "queued", CreatedAt: 1, UpdatedAt: 1,
	}
	if _, _, err := dataStore.CreateTransferJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	_, err = service.Update(ctx, admin.ID, Update{Sources: sourceUpdates("framehdr", "https://source.example")})
	if !errors.Is(err, ErrActiveProviderOperations) {
		t.Fatalf("err = %v", err)
	}
	if applied.Sources != nil {
		t.Fatalf("settings were applied despite active job: %+v", applied)
	}
}

func TestUpdateRejectsBlockingSubXCommand(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	if _, err := dataStore.EnsureAdmin(ctx, "test-hash"); err != nil {
		t.Fatal(err)
	}
	admin, _, err := dataStore.Admin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = dataStore.CreateSubXCommand(ctx, store.SubXCommandJob{
		ID: "subx-command", UserID: admin.ID, OperationID: "framehdr.save", IdempotencyKey: "subx_command",
		RequestHash: []byte("request"), PayloadToken: "encrypted", State: "queued", CreatedAt: 1, UpdatedAt: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	codec, _ := securepayload.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	service := NewService(dataStore, codec, Values{SubX: config.SubX{BaseURL: "https://subx.example", Username: "admin", Password: "secret"}}, nil)
	_, err = service.Update(ctx, admin.ID, Update{Sources: sourceUpdates("", "")})
	if !errors.Is(err, ErrActiveProviderOperations) {
		t.Fatalf("err = %v", err)
	}
}

func TestUpdateAllowsEnablingSameSubXConnectionWithBlockingCommand(t *testing.T) {
	ctx := context.Background()
	dataStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	if _, err := dataStore.EnsureAdmin(ctx, "test-hash"); err != nil {
		t.Fatal(err)
	}
	admin, _, err := dataStore.Admin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = dataStore.CreateSubXCommand(ctx, store.SubXCommandJob{
		ID: "subx-command", UserID: admin.ID, OperationID: "framehdr.save", IdempotencyKey: "subx_command",
		RequestHash: []byte("request"), PayloadToken: "encrypted", State: "queued", CreatedAt: 1, UpdatedAt: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	codec, _ := securepayload.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	initial := Values{SubX: config.SubX{BaseURL: "https://subx.example", Username: "admin", Password: "secret"}}
	var applied Values
	service := NewService(dataStore, codec, initial, func(value Values) { applied = value })
	view, err := service.Update(ctx, admin.ID, Update{
		SubX:    &SubXUpdate{BaseURL: "https://subx.example", Username: "admin", SourceEnabled: true},
		Sources: sourceUpdates("", ""),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !view.SubX.SourceEnabled || !applied.SubX.SourceEnabled {
		t.Fatal("fallback was not enabled")
	}
}

func TestUpdateRejectsIncompleteSubXFallback(t *testing.T) {
	value := Values{SubX: config.SubX{BaseURL: "https://subx.example", SourceEnabled: true}, Sources: configSources()}
	if !errors.Is(validate(value), ErrInvalid) {
		t.Fatal("enabled fallback without credentials was accepted")
	}
	value.SubX.Username = "admin"
	if !errors.Is(validate(value), ErrInvalid) {
		t.Fatal("username without password was accepted")
	}
	value.SubX.Password = "password"
	if err := validate(value); err != nil {
		t.Fatalf("valid fallback rejected: %v", err)
	}
}

func configSources() []config.SearchSource {
	updates := sourceUpdates("", "")
	result := make([]config.SearchSource, 0, len(updates))
	for _, item := range updates {
		result = append(result, config.SearchSource{ID: item.ID, Label: sourceLabels[item.ID]})
	}
	return result
}
