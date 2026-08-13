package settings

import (
	"context"
	"encoding/base64"
	"errors"
	"path/filepath"
	"testing"

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
		Sources:    []config.SearchSource{{ID: "framehdr", Label: "帧影", BaseURL: "https://source.example", Token: "source-token"}},
	}
	var applied Values
	service := NewService(dataStore, codec, initial, func(value Values) { applied = value })
	view, err := service.Update(ctx, 1, Update{
		QMediaSync: QMediaSyncUpdate{BaseURL: "https://qms-new.example"},
		TMDB:       TMDBUpdate{AccessToken: SecretUpdate{}},
		Sources:    sourceUpdates("framehdr", "https://source-new.example"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !view.QMediaSync.APIKey.Configured || !view.TMDB.AccessToken.Configured || !view.Sources[1].Token.Configured {
		t.Fatalf("secret configuration was not preserved: %+v", view)
	}
	if applied.QMediaSync.APIKey != "old-key" || applied.TMDB.BaseURL != "https://api.themoviedb.org/3" || applied.Sources[1].Token != "source-token" {
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
	if values.QMediaSync.APIKey != "old-key" || values.Sources[1].Token != "source-token" {
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
	service := NewService(dataStore, codec, Values{Sources: configSources()}, nil)
	_, err = service.Update(ctx, admin.ID, Update{Sources: sourceUpdates("", "")})
	if !errors.Is(err, ErrActiveProviderOperations) {
		t.Fatalf("err = %v", err)
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

func TestMergePreservesWeComForOlderClients(t *testing.T) {
	current := Values{WeCom: config.WeCom{BaseURL: "https://qyapi.weixin.qq.com", CorpID: "corp", Secret: "secret", ChatID: "chat"}}
	merged := merge(current, Update{Sources: sourceUpdates("", "")})
	if merged.WeCom != current.WeCom {
		t.Fatalf("WeCom settings changed without an explicit update: %+v", merged.WeCom)
	}
}

func TestMergeWeComSecretPreserveAndClear(t *testing.T) {
	current := Values{WeCom: config.WeCom{BaseURL: "https://qyapi.weixin.qq.com", CorpID: "corp", Secret: "secret", ChatID: "chat"}}
	preserved := merge(current, Update{WeCom: &WeComUpdate{BaseURL: "https://qyapi.weixin.qq.com", CorpID: "corp", ChatID: "chat"}})
	if preserved.WeCom.Secret != "secret" {
		t.Fatalf("empty secret update did not preserve secret: %+v", preserved.WeCom)
	}
	cleared := merge(current, Update{WeCom: &WeComUpdate{
		BaseURL: "https://qyapi.weixin.qq.com", CorpID: "corp", Secret: SecretUpdate{Clear: true}, ChatID: "chat",
	}})
	if cleared.WeCom.Secret != "" {
		t.Fatalf("explicit clear did not remove secret: %+v", cleared.WeCom)
	}
}

func TestValidateRejectsPartialWeCom(t *testing.T) {
	value := Values{WeCom: config.WeCom{BaseURL: "https://qyapi.weixin.qq.com", CorpID: "corp"}, Sources: configSources()}
	if !errors.Is(validate(value), ErrInvalid) {
		t.Fatal("partial WeCom was accepted")
	}
	value.WeCom = config.WeCom{BaseURL: "https://qyapi.weixin.qq.com", CorpID: "corp", Secret: "secret", ChatID: "chat"}
	if err := validate(value); err != nil {
		t.Fatalf("complete WeCom rejected: %v", err)
	}
}

func TestMergeDefaultsOfficialWeComURL(t *testing.T) {
	merged := merge(Values{}, Update{WeCom: &WeComUpdate{CorpID: "corp", Secret: SecretUpdate{Value: "secret"}, ChatID: "chat"}})
	if merged.WeCom.BaseURL != "https://qyapi.weixin.qq.com" {
		t.Fatalf("default WeCom URL = %q", merged.WeCom.BaseURL)
	}
	if err := validate(Values{WeCom: merged.WeCom, Sources: configSources()}); err != nil {
		t.Fatalf("defaulted WeCom rejected: %v", err)
	}
}

func TestReadinessCountsBuiltinSourcesWithoutURLs(t *testing.T) {
	service := NewService(nil, nil, Values{Sources: configSources()}, nil)
	_, nativeSources := service.ReadinessConfiguration()
	if nativeSources != 2 {
		t.Fatalf("native sources = %d, want 2", nativeSources)
	}
}
