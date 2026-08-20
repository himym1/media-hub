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
		Sources:    []config.SearchSource{{ID: "framehdr", Label: "帧影", BaseURL: "https://framehdr.com", Account: "old-account", Token: "source-token"}},
	}
	var applied Values
	service := NewService(dataStore, codec, initial, func(value Values) { applied = value })
	view, err := service.Update(ctx, 1, Update{
		QMediaSync: QMediaSyncUpdate{BaseURL: "https://qms-new.example"},
		TMDB:       TMDBUpdate{AccessToken: SecretUpdate{}},
		Sources:    sourceUpdates("framehdr", "https://www.framehdr.com"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !view.QMediaSync.APIKey.Configured || !view.TMDB.AccessToken.Configured || !view.Sources[1].Token.Configured {
		t.Fatalf("secret configuration was not preserved: %+v", view)
	}
	if applied.QMediaSync.APIKey != "old-key" || applied.TMDB.BaseURL != "https://api.themoviedb.org/3" || applied.Sources[1].Account != "old-account" || applied.Sources[1].Token != "source-token" {
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

func TestMergeWeComPreservesNewFieldsForOlderClients(t *testing.T) {
	current := Values{WeCom: config.WeCom{BaseURL: "https://qyapi.weixin.qq.com", CorpID: "corp", Secret: "secret", SendMode: "app", AgentID: 1000005, ToUser: "@all"}}
	merged := merge(current, Update{WeCom: &WeComUpdate{BaseURL: current.WeCom.BaseURL, CorpID: current.WeCom.CorpID}})
	if merged.WeCom != current.WeCom {
		t.Fatalf("older-client WeCom update changed new fields: %+v", merged.WeCom)
	}
}

func TestMergeSourceAccountDistinguishesOmittedAndCleared(t *testing.T) {
	current := Values{Sources: configSources()}
	current.Sources[1].Account = "saved-account"
	omitted := merge(current, Update{Sources: sourceUpdates("", "")})
	if omitted.Sources[1].Account != "saved-account" {
		t.Fatalf("omitted account was not preserved: %+v", omitted.Sources[1])
	}
	empty := ""
	updates := sourceUpdates("", "")
	updates[1].Account = &empty
	cleared := merge(current, Update{Sources: updates})
	if cleared.Sources[1].Account != "" {
		t.Fatalf("explicit empty account was not cleared: %+v", cleared.Sources[1])
	}
}

func TestMergeSourceAuthModeDistinguishesOmittedAndExplicit(t *testing.T) {
	current := Values{Sources: configSources()}
	current.Sources[5].AuthMode = "web"
	current.Sources[5].Account = "user"
	current.Sources[5].Token = "password"
	omittedMode := merge(current, Update{Sources: sourceUpdates("", "")})
	if omittedMode.Sources[5].AuthMode != "web" || omittedMode.Sources[5].Account != "user" || omittedMode.Sources[5].Token != "password" {
		t.Fatalf("omitted auth mode or credentials were not preserved: %+v", omittedMode.Sources[5])
	}
	developer := "developer"
	modeUpdates := sourceUpdates("", "")
	modeUpdates[5].AuthMode = &developer
	changedMode := merge(current, Update{Sources: modeUpdates})
	if changedMode.Sources[5].AuthMode != "developer" || changedMode.Sources[5].Account != "" || changedMode.Sources[5].Token != "" {
		t.Fatalf("mode switch did not clear incompatible credentials: %+v", changedMode.Sources[5])
	}
	blank := publicView(Values{Sources: configSources()})
	if blank.Sources[5].AuthMode != "web" {
		t.Fatalf("blank Juying auth mode = %q", blank.Sources[5].AuthMode)
	}
	legacy := Values{Sources: configSources()}
	legacy.Sources[5].Account = "app-id"
	legacy.Sources[5].Token = "api-key"
	if got := publicView(legacy).Sources[5].AuthMode; got != "developer" {
		t.Fatalf("legacy Juying auth mode = %q", got)
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
	value.WeCom = config.WeCom{BaseURL: "https://qyapi.weixin.qq.com", CorpID: "corp", Secret: "secret", SendMode: "app", AgentID: 1000005, ToUser: "@all"}
	if err := validate(value); err != nil {
		t.Fatalf("complete WeCom application delivery rejected: %v", err)
	}
	value.WeCom.SendMode = "invalid"
	if !errors.Is(validate(value), ErrInvalid) {
		t.Fatal("invalid WeCom send mode was accepted")
	}
}

func TestValidateRequiresCompleteOfficialFrameHDRCredentials(t *testing.T) {
	value := Values{Sources: configSources()}
	value.Sources[1].Account = "user"
	if !errors.Is(validate(value), ErrInvalid) {
		t.Fatal("FrameHDR account without password was accepted")
	}
	value.Sources[1].Token = "pass"
	if err := validate(value); err != nil {
		t.Fatalf("complete FrameHDR credentials rejected: %v", err)
	}
	value.Sources[1].BaseURL = "https://adapter.example/framehdr"
	if !errors.Is(validate(value), ErrInvalid) {
		t.Fatal("native FrameHDR credentials with a contract URL were accepted")
	}
	value.Sources[1].Account = ""
	if err := validate(value); err != nil {
		t.Fatalf("contract FrameHDR token rejected: %v", err)
	}
}

func TestValidateRequiresCompleteOfficialJuyingCredentials(t *testing.T) {
	value := Values{Sources: configSources()}
	value.Sources[5].Account = "app-id"
	if !errors.Is(validate(value), ErrInvalid) {
		t.Fatal("Juying App ID without API Key was accepted")
	}
	value.Sources[5].Token = "app-key"
	if err := validate(value); err != nil {
		t.Fatalf("complete Juying App ID/API Key rejected: %v", err)
	}
	value.Sources[5].BaseURL = "https://adapter.example/juying"
	if !errors.Is(validate(value), ErrInvalid) {
		t.Fatal("native Juying credentials with a contract URL were accepted")
	}
	value.Sources[5].Account = ""
	if err := validate(value); err != nil {
		t.Fatalf("contract Juying token rejected: %v", err)
	}
}

func TestValidateJuyingAuthMode(t *testing.T) {
	value := Values{Sources: configSources()}
	value.Sources[5].Account = "user"
	value.Sources[5].Token = "pass"
	value.Sources[5].AuthMode = "web"
	if err := validate(value); err != nil {
		t.Fatalf("complete Juying web credentials rejected: %v", err)
	}
	value.Sources[5].AuthMode = "invalid"
	if !errors.Is(validate(value), ErrInvalid) {
		t.Fatal("invalid Juying auth mode was accepted")
	}
	value.Sources[5].AuthMode = "web"
	view := publicView(value)
	if view.Sources[5].AuthMode != "web" {
		t.Fatalf("public auth mode = %q", view.Sources[5].AuthMode)
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

func TestMergeKeepsEmbyPasswordUnlessUpdated(t *testing.T) {
	current := Values{Emby: config.Emby{Password: "saved-password"}}
	kept := merge(current, Update{Emby: EmbyUpdate{}})
	if kept.Emby.Password != "saved-password" {
		t.Fatalf("omitted password was not preserved: %q", kept.Emby.Password)
	}
	updated := merge(current, Update{Emby: EmbyUpdate{Password: SecretUpdate{Value: "next-password"}}})
	if updated.Emby.Password != "next-password" {
		t.Fatalf("password was not updated: %q", updated.Emby.Password)
	}
	cleared := merge(current, Update{Emby: EmbyUpdate{Password: SecretUpdate{Clear: true}}})
	if cleared.Emby.Password != "" {
		t.Fatalf("password was not cleared: %q", cleared.Emby.Password)
	}
	view := publicView(Values{Emby: config.Emby{Password: "saved-password"}})
	if !view.Emby.Password.Configured {
		t.Fatal("saved password was not marked configured")
	}
}

func TestReadinessCountsBuiltinSourcesWithoutURLs(t *testing.T) {
	service := NewService(nil, nil, Values{Sources: configSources()}, nil)
	_, nativeSources := service.ReadinessConfiguration()
	if nativeSources != 2 {
		t.Fatalf("native sources = %d, want 2", nativeSources)
	}
	configured := configSources()
	configured[1].Account = "user"
	configured[1].Token = "pass"
	service = NewService(nil, nil, Values{Sources: configured}, nil)
	_, nativeSources = service.ReadinessConfiguration()
	if nativeSources != 3 {
		t.Fatalf("native sources = %d, want 3", nativeSources)
	}
	configured[5].Account = "app-id"
	configured[5].Token = "app-key"
	service = NewService(nil, nil, Values{Sources: configured}, nil)
	_, nativeSources = service.ReadinessConfiguration()
	if nativeSources != 4 {
		t.Fatalf("native sources = %d, want 4", nativeSources)
	}
}
