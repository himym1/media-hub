package settings

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"media-hub/backend/internal/config"
	"media-hub/backend/internal/securepayload"
	"media-hub/backend/internal/store"
)

var ProviderSettingsLock sync.RWMutex

var (
	ErrUnavailable              = errors.New("runtime settings are unavailable")
	ErrInvalid                  = errors.New("runtime settings are invalid")
	ErrActiveProviderOperations = errors.New("provider operations must finish before provider settings can change")
)

type Applier func(Values)

type Service struct {
	store   *store.Store
	codec   *securepayload.Codec
	apply   Applier
	now     func() time.Time
	mutex   sync.RWMutex
	current Values
}

func NewService(dataStore *store.Store, codec *securepayload.Codec, initial Values, apply Applier) *Service {
	service := &Service{store: dataStore, codec: codec, current: clone(initial), apply: apply, now: time.Now}
	if apply != nil {
		apply(clone(initial))
	}
	return service
}

func (s *Service) Load(ctx context.Context, userID int64) error {
	if !s.available() {
		return ErrUnavailable
	}
	sealed, exists, err := s.store.ProviderCredential(ctx, userID, ProviderKey)
	if err != nil {
		return err
	}
	value := s.snapshot()
	if exists {
		if err := s.codec.Open(sealed, &value); err != nil {
			return fmt.Errorf("decrypt runtime settings: %w", err)
		}
		if err := validate(value); err != nil {
			return fmt.Errorf("validate persisted runtime settings: %w", err)
		}
	}
	s.replace(value)
	return nil
}

func (s *Service) Get(context.Context, int64) (View, error) {
	if !s.available() {
		return View{}, ErrUnavailable
	}
	return publicView(s.snapshot()), nil
}

func (s *Service) Update(ctx context.Context, userID int64, input Update) (View, error) {
	if !s.available() {
		return View{}, ErrUnavailable
	}
	ProviderSettingsLock.Lock()
	defer ProviderSettingsLock.Unlock()
	current := s.snapshot()
	value := merge(current, input)
	if err := validate(value); err != nil {
		return View{}, err
	}
	sealed, err := s.codec.Seal(value)
	if err != nil {
		return View{}, err
	}
	if err := s.store.UpsertProviderCredentialWithOperationGuard(ctx, userID, ProviderKey, sealed, false, s.now()); err != nil {
		if errors.Is(err, store.ErrActiveProviderOperations) {
			return View{}, ErrActiveProviderOperations
		}
		return View{}, err
	}
	s.replace(value)
	return publicView(value), nil
}

func (s *Service) Values() Values {
	return s.snapshot()
}

func (s *Service) available() bool {
	return s != nil && s.store != nil && s.codec != nil
}

func (s *Service) snapshot() Values {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return clone(s.current)
}

func (s *Service) replace(value Values) {
	value = clone(value)
	s.mutex.Lock()
	s.current = value
	s.mutex.Unlock()
	if s.apply != nil {
		s.apply(clone(value))
	}
}

func (s *Service) ReadinessConfiguration() (bool, int) {
	value := s.snapshot()
	_, movieReady := value.Workflow.Target("movie")
	_, seriesReady := value.Workflow.Target("series")
	coreReady := value.QMediaSync.BaseURL != "" && value.QMediaSync.APIKey != "" &&
		value.Emby.BaseURL != "" && value.Emby.APIKey != "" &&
		value.TMDB.BaseURL != "" && value.TMDB.AccessToken != "" &&
		movieReady && seriesReady
	nativeSources := 0
	for _, source := range value.Sources {
		if source.BaseURL != "" || isBuiltinSource(source.ID) {
			nativeSources++
		}
	}
	return coreReady, nativeSources
}

func merge(current Values, input Update) Values {
	current.QMediaSync.BaseURL = strings.TrimSpace(input.QMediaSync.BaseURL)
	current.QMediaSync.APIKey = mergeSecret(current.QMediaSync.APIKey, input.QMediaSync.APIKey)
	current.Emby.BaseURL = strings.TrimSpace(input.Emby.BaseURL)
	current.Emby.APIKey = mergeSecret(current.Emby.APIKey, input.Emby.APIKey)
	current.Emby.UserID = strings.TrimSpace(input.Emby.UserID)
	current.Drive115.ClientID = strings.TrimSpace(input.Drive115.ClientID)
	current.TMDB.BaseURL = strings.TrimSpace(input.TMDB.BaseURL)
	current.TMDB.AccessToken = mergeSecret(current.TMDB.AccessToken, input.TMDB.AccessToken)
	if current.TMDB.BaseURL == "" && current.TMDB.AccessToken != "" {
		current.TMDB.BaseURL = "https://api.themoviedb.org/3"
	}
	if input.WeCom != nil {
		current.WeCom.BaseURL = strings.TrimSpace(input.WeCom.BaseURL)
		current.WeCom.CorpID = strings.TrimSpace(input.WeCom.CorpID)
		current.WeCom.Secret = mergeSecret(current.WeCom.Secret, input.WeCom.Secret)
		current.WeCom.ChatID = strings.TrimSpace(input.WeCom.ChatID)
		if current.WeCom.BaseURL == "" && (current.WeCom.CorpID != "" || current.WeCom.Secret != "" || current.WeCom.ChatID != "") {
			current.WeCom.BaseURL = "https://qyapi.weixin.qq.com"
		}
	}
	current.Workflow = input.Workflow.Config()

	byID := make(map[string]config.SearchSource, len(current.Sources))
	for _, source := range current.Sources {
		byID[source.ID] = source
	}
	updated := make([]config.SearchSource, 0, len(input.Sources))
	seen := make(map[string]struct{}, len(input.Sources))
	for _, source := range input.Sources {
		id := strings.TrimSpace(source.ID)
		if _, exists := seen[id]; exists {
			updated = append(updated, config.SearchSource{ID: id})
			continue
		}
		seen[id] = struct{}{}
		previous := byID[id]
		updated = append(updated, config.SearchSource{
			ID: id, Label: sourceLabels[id], BaseURL: strings.TrimSpace(source.BaseURL),
			Token: mergeSecret(previous.Token, source.Token),
		})
	}
	current.Sources = updated
	return current
}

func mergeSecret(current string, update SecretUpdate) string {
	if update.Clear {
		return ""
	}
	if value := strings.TrimSpace(update.Value); value != "" {
		return value
	}
	return current
}

func validate(value Values) error {
	if err := validateURL("QMediaSync URL", value.QMediaSync.BaseURL); err != nil {
		return err
	}
	if err := validateURL("Emby URL", value.Emby.BaseURL); err != nil {
		return err
	}
	if err := validateURL("TMDB URL", value.TMDB.BaseURL); err != nil {
		return err
	}
	if err := validateURL("WeCom URL", value.WeCom.BaseURL); err != nil {
		return err
	}
	if value.Workflow.QMediaSyncAccountID > uint(^uint32(0)) {
		return fmt.Errorf("%w: QMediaSync account ID is out of range", ErrInvalid)
	}
	if len(value.QMediaSync.BaseURL) > 2048 || len(value.QMediaSync.APIKey) > 4096 ||
		len(value.Emby.BaseURL) > 2048 || len(value.Emby.APIKey) > 4096 || len(value.Emby.UserID) > 200 ||
		len(value.Drive115.ClientID) > 200 || len(value.TMDB.BaseURL) > 2048 || len(value.TMDB.AccessToken) > 4096 ||
		len(value.WeCom.BaseURL) > 2048 || len(value.WeCom.CorpID) > 200 || len(value.WeCom.Secret) > 4096 || len(value.WeCom.ChatID) > 200 {
		return fmt.Errorf("%w: provider setting is too long", ErrInvalid)
	}
	wecomFields := []string{value.WeCom.BaseURL, value.WeCom.CorpID, value.WeCom.Secret, value.WeCom.ChatID}
	configuredWeComFields := 0
	for _, field := range wecomFields {
		if field != "" {
			configuredWeComFields++
		}
	}
	if configuredWeComFields != 0 && configuredWeComFields != len(wecomFields) {
		return fmt.Errorf("%w: WeCom URL, corp ID, secret, and chat ID must be configured together", ErrInvalid)
	}
	for _, target := range []config.WorkflowTarget{value.Workflow.Movie, value.Workflow.Series} {
		if len(target.DestinationID) > 200 || len(target.QMediaSyncTargetPath) > 2048 || len(target.EmbyLibraryID) > 200 {
			return fmt.Errorf("%w: workflow target is too long", ErrInvalid)
		}
	}
	if len(value.Sources) != len(sourceLabels) {
		return fmt.Errorf("%w: all source settings are required", ErrInvalid)
	}
	seen := make(map[string]struct{}, len(value.Sources))
	for _, source := range value.Sources {
		if _, ok := sourceLabels[source.ID]; !ok {
			return fmt.Errorf("%w: unsupported source ID", ErrInvalid)
		}
		if _, exists := seen[source.ID]; exists {
			return fmt.Errorf("%w: duplicate source ID", ErrInvalid)
		}
		seen[source.ID] = struct{}{}
		if err := validateURL("source URL", source.BaseURL); err != nil {
			return err
		}
		if source.BaseURL == "" && source.Token != "" && !isBuiltinSource(source.ID) {
			return fmt.Errorf("%w: source URL is required when a token is configured", ErrInvalid)
		}
		if len(source.BaseURL) > 2048 || len(source.Token) > 4096 {
			return fmt.Errorf("%w: source setting is too long", ErrInvalid)
		}
	}
	return nil
}

func validateURL(label, raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("%w: %s must be an absolute HTTP(S) URL without credentials, query, or fragment", ErrInvalid, label)
	}
	return nil
}

func isBuiltinSource(id string) bool {
	return id == "mikan" || id == "sidhub"
}

func publicView(value Values) View {
	sources := make([]SourceView, 0, len(sourceLabels))
	byID := make(map[string]config.SearchSource, len(value.Sources))
	for _, source := range value.Sources {
		byID[source.ID] = source
	}
	for _, id := range []string{"dian", "framehdr", "gimy", "guanying", "hdhive", "juying", "mikan", "sidhub"} {
		source := byID[id]
		sources = append(sources, SourceView{ID: id, Label: sourceLabels[id], BaseURL: source.BaseURL, Token: SecretStatus{Configured: source.Token != ""}})
	}
	return View{
		QMediaSync: QMediaSyncView{BaseURL: value.QMediaSync.BaseURL, APIKey: SecretStatus{Configured: value.QMediaSync.APIKey != ""}},
		Emby:       EmbyView{BaseURL: value.Emby.BaseURL, APIKey: SecretStatus{Configured: value.Emby.APIKey != ""}, UserID: value.Emby.UserID},
		Drive115:   Drive115View{ClientID: value.Drive115.ClientID},
		TMDB:       TMDBView{BaseURL: value.TMDB.BaseURL, AccessToken: SecretStatus{Configured: value.TMDB.AccessToken != ""}},
		WeCom:      WeComView{BaseURL: value.WeCom.BaseURL, CorpID: value.WeCom.CorpID, Secret: SecretStatus{Configured: value.WeCom.Secret != ""}, ChatID: value.WeCom.ChatID},
		Workflow:   workflowFromConfig(value.Workflow),
		Sources:    sources,
	}
}

func clone(value Values) Values {
	value.Sources = append([]config.SearchSource(nil), value.Sources...)
	return value
}
