package settings

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
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
	coreReady := value.Workflow.UsesBuiltinSync() &&
		value.Emby.BaseURL != "" && value.Emby.APIKey != "" &&
		value.TMDB.BaseURL != "" && value.TMDB.AccessToken != "" &&
		movieReady && seriesReady
	nativeSources := 0
	for _, source := range value.Sources {
		if sourceReady(source) {
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
	current.Emby.Password = mergeSecret(current.Emby.Password, input.Emby.Password)
	current.Drive115.ClientID = strings.TrimSpace(input.Drive115.ClientID)
	current.TMDB.BaseURL = strings.TrimSpace(input.TMDB.BaseURL)
	current.TMDB.AccessToken = mergeSecret(current.TMDB.AccessToken, input.TMDB.AccessToken)
	if current.TMDB.BaseURL == "" && current.TMDB.AccessToken != "" {
		current.TMDB.BaseURL = "https://api.themoviedb.org/3"
	}
	if input.Assrt != nil {
		current.Assrt.BaseURL = strings.TrimSpace(input.Assrt.BaseURL)
		current.Assrt.Token = mergeSecret(current.Assrt.Token, input.Assrt.Token)
		if current.Assrt.BaseURL == "" && current.Assrt.Token != "" {
			current.Assrt.BaseURL = "https://api.assrt.net"
		}
	}
	if input.WeCom != nil {
		current.WeCom.BaseURL = strings.TrimSpace(input.WeCom.BaseURL)
		current.WeCom.CorpID = strings.TrimSpace(input.WeCom.CorpID)
		current.WeCom.Secret = mergeSecret(current.WeCom.Secret, input.WeCom.Secret)
		previousMode := current.WeCom.DeliveryMode()
		if input.WeCom.SendMode != nil {
			current.WeCom.SendMode = strings.ToLower(strings.TrimSpace(*input.WeCom.SendMode))
			if current.WeCom.DeliveryMode() != previousMode {
				current.WeCom.AgentID = 0
				current.WeCom.ToUser = ""
				current.WeCom.ChatID = ""
			}
		}
		if input.WeCom.AgentID != nil {
			current.WeCom.AgentID = *input.WeCom.AgentID
		}
		if input.WeCom.ToUser != nil {
			current.WeCom.ToUser = strings.TrimSpace(*input.WeCom.ToUser)
		}
		current.WeCom.ChatID = strings.TrimSpace(input.WeCom.ChatID)
		if current.WeCom.BaseURL == "" && (current.WeCom.CorpID != "" || current.WeCom.Secret != "" || current.WeCom.AgentID != 0 || current.WeCom.ToUser != "" || current.WeCom.ChatID != "") {
			current.WeCom.BaseURL = "https://qyapi.weixin.qq.com"
		}
		sanitizeWeComDelivery(&current.WeCom)
	}
	nextWorkflow := input.Workflow.Config()
	nextWorkflow.SyncMode = config.SyncModeBuiltin
	if nextWorkflow.StrmBaseURL == "" {
		nextWorkflow.StrmBaseURL = current.Workflow.StrmBaseURL
	}
	if nextWorkflow.StrmRootMount == "" {
		nextWorkflow.StrmRootMount = current.Workflow.StrmRootMount
	}
	current.Workflow = nextWorkflow
	if input.CheckIn != nil {
		normalized := NormalizeCheckIn(input.CheckIn)
		current.CheckIn = &normalized
	}

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
		account := previous.Account
		token := previous.Token
		authMode := previous.AuthMode
		if source.AuthMode != nil {
			authMode = strings.ToLower(strings.TrimSpace(*source.AuthMode))
			if id == "juying" && authMode == "" {
				authMode = "web"
			}
			if id == "juying" && authMode != juyingAuthMode(previous) {
				account = ""
				token = ""
			}
		}
		if source.Account != nil {
			account = strings.TrimSpace(*source.Account)
		}
		updated = append(updated, config.SearchSource{
			ID: id, Label: sourceLabels[id], BaseURL: strings.TrimSpace(source.BaseURL), Account: account,
			Token: mergeSecret(token, source.Token), AuthMode: authMode,
		})
	}
	current.Sources = updated
	return current
}

func sanitizeWeComDelivery(value *config.WeCom) {
	if value == nil {
		return
	}
	switch value.DeliveryMode() {
	case config.WeComSendModeApp:
		value.ChatID = ""
	case config.WeComSendModeAppChat:
		value.AgentID = 0
		value.ToUser = ""
	}
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
	if err := validateURL("Assrt URL", value.Assrt.BaseURL); err != nil {
		return err
	}
	if err := validateURL("WeCom URL", value.WeCom.BaseURL); err != nil {
		return err
	}
	if value.Workflow.QMediaSyncAccountID > uint(^uint32(0)) {
		return fmt.Errorf("%w: QMediaSync account ID is out of range", ErrInvalid)
	}
	syncMode := strings.ToLower(strings.TrimSpace(value.Workflow.SyncMode))
	if syncMode != "" && syncMode != config.SyncModeQMediaSync && syncMode != config.SyncModeBuiltin {
		return fmt.Errorf("%w: sync mode must be qmediasync or builtin", ErrInvalid)
	}
	if err := validateURL("STRM base URL", value.Workflow.StrmBaseURL); err != nil {
		return err
	}
	if mount := strings.TrimSpace(value.Workflow.StrmRootMount); mount != "" && !filepath.IsAbs(mount) {
		return fmt.Errorf("%w: STRM root mount must be an absolute path", ErrInvalid)
	}
	if len(value.Workflow.SyncMode) > 20 || len(value.Workflow.StrmBaseURL) > 2048 || len(value.Workflow.StrmRootMount) > 2048 {
		return fmt.Errorf("%w: workflow setting is too long", ErrInvalid)
	}
	checkIn := NormalizeCheckIn(value.CheckIn)
	if checkIn.Hour < 0 || checkIn.Hour > 23 || checkIn.Minute < 0 || checkIn.Minute > 59 {
		return fmt.Errorf("%w: check-in time is invalid", ErrInvalid)
	}
	if len(value.QMediaSync.BaseURL) > 2048 || len(value.QMediaSync.APIKey) > 4096 ||
		len(value.Emby.BaseURL) > 2048 || len(value.Emby.APIKey) > 4096 || len(value.Emby.UserID) > 200 || len(value.Emby.Password) > 4096 ||
		len(value.Drive115.ClientID) > 200 || len(value.TMDB.BaseURL) > 2048 || len(value.TMDB.AccessToken) > 4096 ||
		len(value.Assrt.BaseURL) > 2048 || len(value.Assrt.Token) > 4096 ||
		len(value.WeCom.BaseURL) > 2048 || len(value.WeCom.CorpID) > 200 || len(value.WeCom.Secret) > 4096 || len(value.WeCom.SendMode) > 20 || len(value.WeCom.ToUser) > 200 || len(value.WeCom.ChatID) > 200 {
		return fmt.Errorf("%w: provider setting is too long", ErrInvalid)
	}
	if value.WeCom.AgentID > uint(^uint32(0)) {
		return fmt.Errorf("%w: WeCom agent ID is out of range", ErrInvalid)
	}
	configuredWeCom := value.WeCom.BaseURL != "" || value.WeCom.CorpID != "" || value.WeCom.Secret != "" || value.WeCom.AgentID != 0 || value.WeCom.ToUser != "" || value.WeCom.ChatID != ""
	if configuredWeCom {
		if value.WeCom.BaseURL == "" || value.WeCom.CorpID == "" || value.WeCom.Secret == "" {
			return fmt.Errorf("%w: WeCom URL, corp ID, and secret must be configured together", ErrInvalid)
		}
		switch value.WeCom.DeliveryMode() {
		case config.WeComSendModeApp:
			if value.WeCom.AgentID == 0 || value.WeCom.ToUser == "" || value.WeCom.ChatID != "" {
				return fmt.Errorf("%w: WeCom app mode requires agent ID and recipient without a chat ID", ErrInvalid)
			}
		case config.WeComSendModeAppChat:
			if value.WeCom.ChatID == "" || value.WeCom.AgentID != 0 || value.WeCom.ToUser != "" {
				return fmt.Errorf("%w: WeCom appchat mode requires a chat ID without agent ID or recipient", ErrInvalid)
			}
		default:
			return fmt.Errorf("%w: WeCom send mode must be app or appchat", ErrInvalid)
		}
	} else if mode := value.WeCom.DeliveryMode(); mode != config.WeComSendModeApp && mode != config.WeComSendModeAppChat {
		return fmt.Errorf("%w: WeCom send mode must be app or appchat", ErrInvalid)
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
		if source.ID == "framehdr" {
			native := source.BaseURL == "" || isOfficialSourceRoot(source.BaseURL, "framehdr.com")
			if native && (source.Account == "") != (source.Token == "") {
				return fmt.Errorf("%w: FrameHDR username and password must be configured together", ErrInvalid)
			}
			if !native && source.Account != "" {
				return fmt.Errorf("%w: FrameHDR account credentials require the official source URL", ErrInvalid)
			}
		}
		if source.ID == "juying" {
			native := source.BaseURL == "" || isOfficialSourceRoot(source.BaseURL, "jying.top")
			mode := juyingAuthMode(source)
			if native && mode != "web" && mode != "developer" {
				return fmt.Errorf("%w: Juying auth mode must be web or developer", ErrInvalid)
			}
			if native && (source.Account == "") != (source.Token == "") {
				return fmt.Errorf("%w: Juying account and secret must be configured together", ErrInvalid)
			}
			if !native && (source.Account != "" || source.AuthMode != "") {
				return fmt.Errorf("%w: Juying account credentials require the official source URL", ErrInvalid)
			}
		}
		if source.Account != "" && source.ID != "framehdr" && source.ID != "juying" {
			return fmt.Errorf("%w: source account is not supported", ErrInvalid)
		}
		if source.AuthMode != "" && source.ID != "juying" {
			return fmt.Errorf("%w: source auth mode is not supported", ErrInvalid)
		}
		if len(source.BaseURL) > 2048 || len(source.Account) > 200 || len(source.Token) > 4096 || len(source.AuthMode) > 20 {
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
	return id == "mikan" || id == "sidhub" || id == "framehdr" || id == "juying"
}

func sourceReady(source config.SearchSource) bool {
	switch source.ID {
	case "mikan", "sidhub":
		return true
	case "framehdr":
		if source.BaseURL == "" || isOfficialSourceRoot(source.BaseURL, "framehdr.com") {
			return source.Account != "" && source.Token != ""
		}
		return source.BaseURL != ""
	case "juying":
		if source.BaseURL == "" || isOfficialSourceRoot(source.BaseURL, "jying.top") {
			mode := juyingAuthMode(source)
			return (mode == "web" || mode == "developer") && source.Account != "" && source.Token != ""
		}
		return source.BaseURL != ""
	default:
		return source.BaseURL != ""
	}
}

func juyingAuthMode(source config.SearchSource) string {
	if mode := strings.ToLower(strings.TrimSpace(source.AuthMode)); mode != "" {
		return mode
	}
	if source.Account != "" || source.Token != "" {
		return "developer"
	}
	return "web"
}

func isOfficialSourceRoot(raw, host string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Port() != "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	name := strings.ToLower(parsed.Hostname())
	return name == host || name == "www."+host
}

func publicView(value Values) View {
	sources := make([]SourceView, 0, len(sourceLabels))
	byID := make(map[string]config.SearchSource, len(value.Sources))
	for _, source := range value.Sources {
		byID[source.ID] = source
	}
	for _, id := range []string{"dian", "framehdr", "gimy", "guanying", "hdhive", "juying", "mikan", "sidhub"} {
		source := byID[id]
		authMode := ""
		if id == "juying" {
			authMode = juyingAuthMode(source)
		}
		sources = append(sources, SourceView{ID: id, Label: sourceLabels[id], BaseURL: source.BaseURL, Account: source.Account, AuthMode: authMode, Token: SecretStatus{Configured: source.Token != ""}})
	}
	return View{
		QMediaSync: QMediaSyncView{BaseURL: value.QMediaSync.BaseURL, APIKey: SecretStatus{Configured: value.QMediaSync.APIKey != ""}},
		Emby: EmbyView{
			BaseURL: value.Emby.BaseURL, APIKey: SecretStatus{Configured: value.Emby.APIKey != ""},
			UserID: value.Emby.UserID, Password: SecretStatus{Configured: value.Emby.Password != ""},
		},
		Drive115: Drive115View{ClientID: value.Drive115.ClientID},
		TMDB:     TMDBView{BaseURL: value.TMDB.BaseURL, AccessToken: SecretStatus{Configured: value.TMDB.AccessToken != ""}},
		Assrt:    AssrtView{BaseURL: value.Assrt.BaseURL, Token: SecretStatus{Configured: value.Assrt.Token != ""}},
		WeCom: WeComView{
			BaseURL: value.WeCom.BaseURL, CorpID: value.WeCom.CorpID, Secret: SecretStatus{Configured: value.WeCom.Secret != ""},
			SendMode: value.WeCom.DeliveryMode(), AgentID: value.WeCom.AgentID, ToUser: value.WeCom.ToUser, ChatID: value.WeCom.ChatID,
		},
		Workflow: workflowFromConfig(value.Workflow),
		CheckIn:  NormalizeCheckIn(value.CheckIn),
		Sources:  sources,
	}
}

func clone(value Values) Values {
	value.Sources = append([]config.SearchSource(nil), value.Sources...)
	if value.CheckIn != nil {
		copied := *value.CheckIn
		copied.Sources = append([]string(nil), value.CheckIn.Sources...)
		value.CheckIn = &copied
	}
	return value
}
