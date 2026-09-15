package settings

import (
	"strings"

	"media-hub/backend/internal/config"
)

const ProviderKey = "media-hub-runtime-settings"

type Values struct {
	QMediaSync config.QMediaSync     `json:"qmediaSync"`
	Emby       config.Emby           `json:"emby"`
	Drive115   config.Drive115       `json:"drive115"`
	TMDB       config.TMDB           `json:"tmdb"`
	Assrt      config.Assrt          `json:"assrt"`
	WeCom      config.WeCom          `json:"wecom"`
	Workflow   config.Workflow       `json:"workflow"`
	CheckIn    *CheckInSettings      `json:"checkIn,omitempty"`
	Sources    []config.SearchSource `json:"sources"`
}

type CheckInSettings struct {
	Enabled bool     `json:"enabled"`
	Hour    int      `json:"hour"`
	Minute  int      `json:"minute"`
	Sources []string `json:"sources"`
}

type WorkflowTarget struct {
	DestinationID        string `json:"destinationId"`
	QMediaSyncTargetPath string `json:"qMediaSyncTargetPath"`
	EmbyLibraryID        string `json:"embyLibraryId"`
}

type Workflow struct {
	SyncMode            string         `json:"syncMode"`
	StrmBaseURL         string         `json:"strmBaseUrl"`
	StrmRootMount       string         `json:"strmRootMount"`
	QMediaSyncAccountID uint           `json:"qMediaSyncAccountId"`
	Movie               WorkflowTarget `json:"movie"`
	Series              WorkflowTarget `json:"series"`
}

func workflowFromConfig(value config.Workflow) Workflow {
	return Workflow{
		SyncMode:            value.NormalizedSyncMode(),
		StrmBaseURL:         value.StrmBaseURL,
		StrmRootMount:       value.StrmRootMount,
		QMediaSyncAccountID: value.QMediaSyncAccountID,
		Movie:               WorkflowTarget{DestinationID: value.Movie.DestinationID, QMediaSyncTargetPath: value.Movie.QMediaSyncTargetPath, EmbyLibraryID: value.Movie.EmbyLibraryID},
		Series:              WorkflowTarget{DestinationID: value.Series.DestinationID, QMediaSyncTargetPath: value.Series.QMediaSyncTargetPath, EmbyLibraryID: value.Series.EmbyLibraryID},
	}
}

func (value Workflow) Config() config.Workflow {
	return config.Workflow{
		SyncMode:            strings.ToLower(strings.TrimSpace(value.SyncMode)),
		StrmBaseURL:         strings.TrimSpace(value.StrmBaseURL),
		StrmRootMount:       strings.TrimSpace(value.StrmRootMount),
		QMediaSyncAccountID: value.QMediaSyncAccountID,
		Movie:               config.WorkflowTarget{DestinationID: value.Movie.DestinationID, QMediaSyncTargetPath: value.Movie.QMediaSyncTargetPath, EmbyLibraryID: value.Movie.EmbyLibraryID},
		Series:              config.WorkflowTarget{DestinationID: value.Series.DestinationID, QMediaSyncTargetPath: value.Series.QMediaSyncTargetPath, EmbyLibraryID: value.Series.EmbyLibraryID},
	}
}

type SecretUpdate struct {
	Value string `json:"value"`
	Clear bool   `json:"clear"`
}

type QMediaSyncUpdate struct {
	BaseURL string       `json:"baseUrl"`
	APIKey  SecretUpdate `json:"apiKey"`
}

type EmbyUpdate struct {
	BaseURL  string       `json:"baseUrl"`
	APIKey   SecretUpdate `json:"apiKey"`
	UserID   string       `json:"userId"`
	Password SecretUpdate `json:"password"`
}

type Drive115Update struct {
	ClientID string `json:"clientId"`
}

type TMDBUpdate struct {
	BaseURL     string       `json:"baseUrl"`
	AccessToken SecretUpdate `json:"accessToken"`
}

type AssrtUpdate struct {
	BaseURL string       `json:"baseUrl"`
	Token   SecretUpdate `json:"token"`
}

type SourceUpdate struct {
	ID       string       `json:"id"`
	BaseURL  string       `json:"baseUrl"`
	Account  *string      `json:"account,omitempty"`
	AuthMode *string      `json:"authMode,omitempty"`
	Token    SecretUpdate `json:"token"`
}

type WeComUpdate struct {
	BaseURL  string       `json:"baseUrl"`
	CorpID   string       `json:"corpId"`
	Secret   SecretUpdate `json:"secret"`
	SendMode *string      `json:"sendMode,omitempty"`
	AgentID  *uint        `json:"agentId,omitempty"`
	ToUser   *string      `json:"toUser,omitempty"`
	ChatID   string       `json:"chatId"`
}

type Update struct {
	QMediaSync QMediaSyncUpdate  `json:"qmediaSync"`
	Emby       EmbyUpdate        `json:"emby"`
	Drive115   Drive115Update    `json:"drive115"`
	TMDB       TMDBUpdate        `json:"tmdb"`
	Assrt      *AssrtUpdate      `json:"assrt"`
	WeCom      *WeComUpdate      `json:"wecom"`
	Workflow   Workflow          `json:"workflow"`
	CheckIn    *CheckInSettings  `json:"checkIn,omitempty"`
	Sources    []SourceUpdate    `json:"sources"`
}

type SecretStatus struct {
	Configured bool `json:"configured"`
}

type QMediaSyncView struct {
	BaseURL string       `json:"baseUrl"`
	APIKey  SecretStatus `json:"apiKey"`
}

type EmbyView struct {
	BaseURL  string       `json:"baseUrl"`
	APIKey   SecretStatus `json:"apiKey"`
	UserID   string       `json:"userId"`
	Password SecretStatus `json:"password"`
}

type Drive115View struct {
	ClientID string `json:"clientId"`
}

type TMDBView struct {
	BaseURL     string       `json:"baseUrl"`
	AccessToken SecretStatus `json:"accessToken"`
}

type AssrtView struct {
	BaseURL string       `json:"baseUrl"`
	Token   SecretStatus `json:"token"`
}

type SourceView struct {
	ID       string       `json:"id"`
	Label    string       `json:"label"`
	BaseURL  string       `json:"baseUrl"`
	Account  string       `json:"account"`
	AuthMode string       `json:"authMode"`
	Token    SecretStatus `json:"token"`
}

type WeComView struct {
	BaseURL  string       `json:"baseUrl"`
	CorpID   string       `json:"corpId"`
	Secret   SecretStatus `json:"secret"`
	SendMode string       `json:"sendMode"`
	AgentID  uint         `json:"agentId"`
	ToUser   string       `json:"toUser"`
	ChatID   string       `json:"chatId"`
}

type View struct {
	QMediaSync QMediaSyncView  `json:"qmediaSync"`
	Emby       EmbyView        `json:"emby"`
	Drive115   Drive115View    `json:"drive115"`
	TMDB       TMDBView        `json:"tmdb"`
	Assrt      AssrtView       `json:"assrt"`
	WeCom      WeComView       `json:"wecom"`
	Workflow   Workflow        `json:"workflow"`
	CheckIn    CheckInSettings `json:"checkIn"`
	Sources    []SourceView    `json:"sources"`
}

var sourceLabels = map[string]string{
	"dian": "点点", "framehdr": "帧影", "gimy": "Gimy", "guanying": "观影",
	"hdhive": "HDHive", "juying": "聚影", "mikan": "蜜柑", "sidhub": "Sidhub",
}

func DefaultCheckIn() CheckInSettings {
	return CheckInSettings{Enabled: true, Hour: 0, Minute: 5, Sources: []string{"framehdr", "juying"}}
}

func NormalizeCheckIn(value *CheckInSettings) CheckInSettings {
	if value == nil {
		return DefaultCheckIn()
	}
	normalized := *value
	normalized.Sources = normalizeCheckInSources(normalized.Sources)
	return normalized
}

func normalizeCheckInSources(values []string) []string {
	if values == nil {
		return []string{"framehdr", "juying"}
	}
	allowed := map[string]struct{}{"framehdr": {}, "juying": {}}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		id := strings.TrimSpace(value)
		if _, ok := allowed[id]; !ok {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func FromConfig(value config.Config) Values {
	return Values{
		QMediaSync: value.QMediaSync,
		Emby:       value.Emby,
		Drive115:   value.Drive115,
		TMDB:       value.TMDB,
		Assrt:      value.Assrt,
		WeCom:      value.WeCom,
		Workflow:   value.Workflow,
		Sources:    append([]config.SearchSource(nil), value.Sources...),
	}
}
