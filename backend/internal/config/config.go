package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAddress      = ":8080"
	defaultDatabasePath = "data/media-hub.db"
	defaultProbeTimeout = 3 * time.Second
)

type Config struct {
	Address                string
	DatabasePath           string
	ProbeTimeout           time.Duration
	SourceProxyURL         *url.URL
	FixtureMode            bool
	SecureCookies          bool
	BootstrapAdminPassword string
	DataEncryptionKey      string
	QMediaSync             QMediaSync
	Emby                   Emby
	Drive115               Drive115
	TMDB                   TMDB
	WeCom                  WeCom
	Workflow               Workflow
	Sources                []SearchSource
	LocalUploadRoots       []string
	AndroidReleaseDir      string
	Integrations           []Integration
}

type QMediaSync struct {
	BaseURL string
	APIKey  string
}

type Emby struct {
	BaseURL string
	APIKey  string
	UserID  string
}

type Drive115 struct {
	AccessToken string
	ClientID    string
}

type TMDB struct {
	BaseURL     string
	AccessToken string
}

type WeCom struct {
	BaseURL  string
	CorpID   string
	Secret   string
	SendMode string
	AgentID  uint
	ToUser   string
	ChatID   string
}

const (
	WeComSendModeApp     = "app"
	WeComSendModeAppChat = "appchat"
)

func (configuration WeCom) DeliveryMode() string {
	if mode := strings.ToLower(strings.TrimSpace(configuration.SendMode)); mode != "" {
		return mode
	}
	if configuration.ChatID != "" {
		return WeComSendModeAppChat
	}
	return WeComSendModeApp
}

type Workflow struct {
	QMediaSyncAccountID uint
	Movie               WorkflowTarget
	Series              WorkflowTarget
}

type WorkflowTarget struct {
	DestinationID        string
	QMediaSyncTargetPath string
	EmbyLibraryID        string
}

func (w Workflow) Target(mediaType string) (WorkflowTarget, bool) {
	if w.QMediaSyncAccountID == 0 {
		return WorkflowTarget{}, false
	}
	var target WorkflowTarget
	switch mediaType {
	case "movie":
		target = w.Movie
	case "series":
		target = w.Series
	default:
		return WorkflowTarget{}, false
	}
	return target, target.DestinationID != "" && target.QMediaSyncTargetPath != "" && target.EmbyLibraryID != ""
}

type SearchSource struct {
	ID       string
	Label    string
	BaseURL  string
	Account  string
	Token    string
	AuthMode string
}

type Integration struct {
	ID      string
	Label   string
	BaseURL string
}

func Load() (Config, error) {
	return load(os.LookupEnv)
}

func load(lookup func(string) (string, bool)) (Config, error) {
	probeTimeout, err := durationValue(lookup, "MEDIA_HUB_PROBE_TIMEOUT", defaultProbeTimeout)
	if err != nil {
		return Config{}, err
	}
	sourceProxyURL, err := proxyURLValue(lookup, "MEDIA_HUB_SOURCE_PROXY_URL")
	if err != nil {
		return Config{}, err
	}
	if sourceProxyURL == nil {
		sourceProxyURL, err = proxyURLValue(lookup, "MEDIA_HUB_MIKAN_PROXY_URL")
		if err != nil {
			return Config{}, err
		}
	}
	fixtureMode, err := boolValue(lookup, "MEDIA_HUB_ENABLE_FIXTURES", false)
	if err != nil {
		return Config{}, err
	}
	secureCookies, err := boolValue(lookup, "MEDIA_HUB_SECURE_COOKIES", false)
	if err != nil {
		return Config{}, err
	}
	qmsAccountID, err := uintValue(lookup, "MEDIA_HUB_QMS_ACCOUNT_ID")
	if err != nil {
		return Config{}, err
	}
	adminPassword := secretValue(lookup, "MEDIA_HUB_ADMIN_PASSWORD")
	if adminPassword != "" && (len(adminPassword) < 12 || len(adminPassword) > 1024) {
		return Config{}, fmt.Errorf("MEDIA_HUB_ADMIN_PASSWORD must be between 12 and 1024 bytes")
	}
	localUploadRoots, err := parseLocalUploadRoots(stringValue(lookup, "MEDIA_HUB_LOCAL_UPLOAD_ROOTS", ""))
	if err != nil {
		return Config{}, err
	}
	androidReleaseDir := stringValue(lookup, "MEDIA_HUB_ANDROID_RELEASE_DIR", "")
	if androidReleaseDir != "" {
		androidReleaseDir = filepath.Clean(androidReleaseDir)
		if !filepath.IsAbs(androidReleaseDir) {
			return Config{}, fmt.Errorf("MEDIA_HUB_ANDROID_RELEASE_DIR must be an absolute path")
		}
	}

	qmsURL, err := baseURLValue(lookup, "MEDIA_HUB_QMS_URL")
	if err != nil {
		return Config{}, err
	}
	embyURL, err := baseURLValue(lookup, "MEDIA_HUB_EMBY_URL")
	if err != nil {
		return Config{}, err
	}
	tmdbURL, err := baseURLValue(lookup, "MEDIA_HUB_TMDB_URL")
	if err != nil {
		return Config{}, err
	}
	wecomURL, err := baseURLValue(lookup, "MEDIA_HUB_WECOM_URL")
	if err != nil {
		return Config{}, err
	}
	wecomAgentID, err := uintValue(lookup, "MEDIA_HUB_WECOM_AGENT_ID")
	if err != nil {
		return Config{}, err
	}
	frameURL, err := baseURLValue(lookup, "MEDIA_HUB_SOURCE_FRAME_URL")
	if err != nil {
		return Config{}, err
	}
	gatherURL, err := baseURLValue(lookup, "MEDIA_HUB_SOURCE_GATHER_URL")
	if err != nil {
		return Config{}, err
	}

	qms := QMediaSync{BaseURL: qmsURL, APIKey: secretValue(lookup, "MEDIA_HUB_QMS_API_KEY")}
	emby := Emby{
		BaseURL: embyURL,
		APIKey:  secretValue(lookup, "MEDIA_HUB_EMBY_API_KEY"),
		UserID:  stringValue(lookup, "MEDIA_HUB_EMBY_USER_ID", ""),
	}
	drive115 := Drive115{
		AccessToken: secretValue(lookup, "MEDIA_HUB_115_ACCESS_TOKEN"),
		ClientID:    stringValue(lookup, "MEDIA_HUB_115_CLIENT_ID", ""),
	}
	tmdbToken := secretValue(lookup, "MEDIA_HUB_TMDB_ACCESS_TOKEN")
	if tmdbURL == "" && tmdbToken != "" {
		tmdbURL = "https://api.themoviedb.org/3"
	}
	tmdbConfig := TMDB{BaseURL: tmdbURL, AccessToken: tmdbToken}
	wecomConfig := WeCom{
		BaseURL:  wecomURL,
		CorpID:   strings.TrimSpace(stringValue(lookup, "MEDIA_HUB_WECOM_CORP_ID", "")),
		Secret:   secretValue(lookup, "MEDIA_HUB_WECOM_SECRET"),
		SendMode: strings.ToLower(strings.TrimSpace(stringValue(lookup, "MEDIA_HUB_WECOM_SEND_MODE", ""))),
		AgentID:  wecomAgentID,
		ToUser:   strings.TrimSpace(stringValue(lookup, "MEDIA_HUB_WECOM_TO_USER", "")),
		ChatID:   strings.TrimSpace(stringValue(lookup, "MEDIA_HUB_WECOM_CHAT_ID", "")),
	}
	if wecomConfig.SendMode == "" {
		wecomConfig.SendMode = wecomConfig.DeliveryMode()
	}
	if wecomConfig.BaseURL == "" && (wecomConfig.CorpID != "" || wecomConfig.Secret != "" || wecomConfig.AgentID != 0 || wecomConfig.ToUser != "" || wecomConfig.ChatID != "") {
		wecomConfig.BaseURL = "https://qyapi.weixin.qq.com"
	}
	if err := validateWeCom(wecomConfig); err != nil {
		return Config{}, err
	}
	sources, err := searchSourceConfigurations(lookup, frameURL, gatherURL)
	if err != nil {
		return Config{}, err
	}

	drive115URL := ""
	if drive115.AccessToken != "" {
		drive115URL = "https://proapi.115.com"
	}

	return Config{
		Address:                stringValue(lookup, "MEDIA_HUB_ADDR", defaultAddress),
		DatabasePath:           stringValue(lookup, "MEDIA_HUB_DATABASE_PATH", defaultDatabasePath),
		ProbeTimeout:           probeTimeout,
		SourceProxyURL:         sourceProxyURL,
		FixtureMode:            fixtureMode,
		SecureCookies:          secureCookies,
		BootstrapAdminPassword: adminPassword,
		DataEncryptionKey:      secretValue(lookup, "MEDIA_HUB_DATA_ENCRYPTION_KEY"),
		QMediaSync:             qms,
		Emby:                   emby,
		Drive115:               drive115,
		TMDB:                   tmdbConfig,
		WeCom:                  wecomConfig,
		Workflow: Workflow{
			QMediaSyncAccountID: qmsAccountID,
			Movie: WorkflowTarget{
				DestinationID:        stringValue(lookup, "MEDIA_HUB_115_MOVIE_DESTINATION_ID", ""),
				QMediaSyncTargetPath: stringValue(lookup, "MEDIA_HUB_QMS_MOVIE_TARGET_PATH", ""),
				EmbyLibraryID:        stringValue(lookup, "MEDIA_HUB_EMBY_MOVIE_LIBRARY_ID", ""),
			},
			Series: WorkflowTarget{
				DestinationID:        stringValue(lookup, "MEDIA_HUB_115_SERIES_DESTINATION_ID", ""),
				QMediaSyncTargetPath: stringValue(lookup, "MEDIA_HUB_QMS_SERIES_TARGET_PATH", ""),
				EmbyLibraryID:        stringValue(lookup, "MEDIA_HUB_EMBY_SERIES_LIBRARY_ID", ""),
			},
		},
		Sources:           sources,
		LocalUploadRoots:  localUploadRoots,
		AndroidReleaseDir: androidReleaseDir,
		Integrations: []Integration{
			{ID: "sources", Label: "资源源"},
			{ID: "115", Label: "115", BaseURL: drive115URL},
			{ID: "tmdb", Label: "TMDB", BaseURL: tmdbConfig.BaseURL},
			{ID: "wecom", Label: "企业微信", BaseURL: wecomConfig.BaseURL},
			{ID: "qmediasync", Label: "QMediaSync", BaseURL: qms.BaseURL},
			{ID: "emby", Label: "Emby", BaseURL: emby.BaseURL},
		},
	}, nil
}

func validateWeCom(configuration WeCom) error {
	mode := configuration.DeliveryMode()
	if mode != WeComSendModeApp && mode != WeComSendModeAppChat {
		return fmt.Errorf("MEDIA_HUB_WECOM_SEND_MODE must be app or appchat")
	}
	if configuration.AgentID > uint(^uint32(0)) {
		return fmt.Errorf("MEDIA_HUB_WECOM_AGENT_ID is out of range")
	}
	configured := configuration.BaseURL != "" || configuration.CorpID != "" || configuration.Secret != "" || configuration.AgentID != 0 || configuration.ToUser != "" || configuration.ChatID != ""
	if !configured {
		return nil
	}
	if configuration.BaseURL == "" || configuration.CorpID == "" || configuration.Secret == "" {
		return fmt.Errorf("WeCom URL, corp ID, and secret must be configured together")
	}
	switch mode {
	case WeComSendModeApp:
		if configuration.AgentID == 0 || configuration.ToUser == "" || configuration.ChatID != "" {
			return fmt.Errorf("WeCom app mode requires agent ID and recipient without a chat ID")
		}
	case WeComSendModeAppChat:
		if configuration.ChatID == "" || configuration.AgentID != 0 || configuration.ToUser != "" {
			return fmt.Errorf("WeCom appchat mode requires a chat ID without agent ID or recipient")
		}
	}
	return nil
}

func parseLocalUploadRoots(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return []string{}, nil
	}
	values := make([]string, 0)
	seen := map[string]struct{}{}
	for _, item := range strings.Split(raw, ",") {
		value := filepath.Clean(strings.TrimSpace(item))
		if !filepath.IsAbs(value) {
			return nil, fmt.Errorf("MEDIA_HUB_LOCAL_UPLOAD_ROOTS entries must be absolute paths")
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values, nil
}

func stringValue(lookup func(string) (string, bool), key, fallback string) string {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func secretValue(lookup func(string) (string, bool), key string) string {
	value, ok := lookup(key)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func durationValue(lookup func(string) (string, bool), key string, fallback time.Duration) (time.Duration, error) {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration < 100*time.Millisecond || duration > 30*time.Second {
		return 0, fmt.Errorf("%s must be a duration between 100ms and 30s", key)
	}
	return duration, nil
}

func searchSourceConfigurations(
	lookup func(string) (string, bool), legacyFrameURL, legacyGatherURL string,
) ([]SearchSource, error) {
	specifications := []struct {
		id    string
		label string
		env   string
	}{
		{"dian", "点点", "DIAN"},
		{"framehdr", "帧影", "FRAMEHDR"},
		{"gimy", "Gimy", "GIMY"},
		{"guanying", "观影", "GUANYING"},
		{"hdhive", "HDHive", "HDHIVE"},
		{"juying", "聚影", "JUYING"},
		{"mikan", "蜜柑", "MIKAN"},
		{"sidhub", "Sidhub", "SIDHUB"},
	}
	result := make([]SearchSource, 0, len(specifications))
	for _, specification := range specifications {
		baseURL, err := baseURLValue(lookup, "MEDIA_HUB_SOURCE_"+specification.env+"_URL")
		if err != nil {
			return nil, err
		}
		token := secretValue(lookup, "MEDIA_HUB_SOURCE_"+specification.env+"_TOKEN")
		if specification.id == "framehdr" && baseURL == "" {
			baseURL = legacyFrameURL
			token = firstNonEmpty(token, secretValue(lookup, "MEDIA_HUB_SOURCE_FRAME_TOKEN"))
		}
		if specification.id == "juying" && baseURL == "" {
			baseURL = legacyGatherURL
			token = firstNonEmpty(token, secretValue(lookup, "MEDIA_HUB_SOURCE_GATHER_TOKEN"))
		}
		account := stringValue(lookup, "MEDIA_HUB_SOURCE_"+specification.env+"_ACCOUNT", "")
		authMode := strings.ToLower(strings.TrimSpace(stringValue(lookup, "MEDIA_HUB_SOURCE_"+specification.env+"_AUTH_MODE", "")))
		if authMode != "" && (specification.id != "juying" || (authMode != "web" && authMode != "developer")) {
			return nil, fmt.Errorf("MEDIA_HUB_SOURCE_%s_AUTH_MODE must be web or developer", specification.env)
		}
		if baseURL != "" || account != "" || token != "" || authMode != "" {
			result = append(result, SearchSource{ID: specification.id, Label: specification.label, BaseURL: baseURL, Account: account, Token: token, AuthMode: authMode})
		}
	}
	return result, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func boolValue(lookup func(string) (string, bool), key string, fallback bool) (bool, error) {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}
	return parsed, nil
}

func uintValue(lookup func(string) (string, bool), key string) (uint, error) {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 32)
	if err != nil || parsed == 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return uint(parsed), nil
}

func proxyURLValue(lookup func(string) (string, bool), key string) (*url.URL, error) {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("%s must be an absolute HTTP(S) URL", key)
	}
	if parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("%s must not contain credentials, a path, query parameters, or fragments", key)
	}
	parsed.Path = ""
	return parsed, nil
}

func baseURLValue(lookup func(string) (string, bool), key string) (string, error) {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return "", nil
	}
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("%s must be an absolute HTTP(S) URL", key)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("%s must not contain credentials, query parameters, or fragments", key)
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}
