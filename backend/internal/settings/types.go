package settings

import "media-hub/backend/internal/config"

const ProviderKey = "media-hub-runtime-settings"

type Values struct {
	QMediaSync config.QMediaSync     `json:"qmediaSync"`
	Emby       config.Emby           `json:"emby"`
	Drive115   config.Drive115       `json:"drive115"`
	TMDB       config.TMDB           `json:"tmdb"`
	Workflow   config.Workflow       `json:"workflow"`
	Sources    []config.SearchSource `json:"sources"`
}

type WorkflowTarget struct {
	DestinationID        string `json:"destinationId"`
	QMediaSyncTargetPath string `json:"qMediaSyncTargetPath"`
	EmbyLibraryID        string `json:"embyLibraryId"`
}

type Workflow struct {
	QMediaSyncAccountID uint           `json:"qMediaSyncAccountId"`
	Movie               WorkflowTarget `json:"movie"`
	Series              WorkflowTarget `json:"series"`
}

func workflowFromConfig(value config.Workflow) Workflow {
	return Workflow{
		QMediaSyncAccountID: value.QMediaSyncAccountID,
		Movie:               WorkflowTarget{DestinationID: value.Movie.DestinationID, QMediaSyncTargetPath: value.Movie.QMediaSyncTargetPath, EmbyLibraryID: value.Movie.EmbyLibraryID},
		Series:              WorkflowTarget{DestinationID: value.Series.DestinationID, QMediaSyncTargetPath: value.Series.QMediaSyncTargetPath, EmbyLibraryID: value.Series.EmbyLibraryID},
	}
}

func (value Workflow) Config() config.Workflow {
	return config.Workflow{
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
	BaseURL string       `json:"baseUrl"`
	APIKey  SecretUpdate `json:"apiKey"`
	UserID  string       `json:"userId"`
}

type Drive115Update struct {
	ClientID string `json:"clientId"`
}

type TMDBUpdate struct {
	BaseURL     string       `json:"baseUrl"`
	AccessToken SecretUpdate `json:"accessToken"`
}

type SourceUpdate struct {
	ID      string       `json:"id"`
	BaseURL string       `json:"baseUrl"`
	Token   SecretUpdate `json:"token"`
}

type Update struct {
	QMediaSync QMediaSyncUpdate `json:"qmediaSync"`
	Emby       EmbyUpdate       `json:"emby"`
	Drive115   Drive115Update   `json:"drive115"`
	TMDB       TMDBUpdate       `json:"tmdb"`
	Workflow   Workflow         `json:"workflow"`
	Sources    []SourceUpdate   `json:"sources"`
}

type SecretStatus struct {
	Configured bool `json:"configured"`
}

type QMediaSyncView struct {
	BaseURL string       `json:"baseUrl"`
	APIKey  SecretStatus `json:"apiKey"`
}

type EmbyView struct {
	BaseURL string       `json:"baseUrl"`
	APIKey  SecretStatus `json:"apiKey"`
	UserID  string       `json:"userId"`
}

type Drive115View struct {
	ClientID string `json:"clientId"`
}

type TMDBView struct {
	BaseURL     string       `json:"baseUrl"`
	AccessToken SecretStatus `json:"accessToken"`
}

type SourceView struct {
	ID      string       `json:"id"`
	Label   string       `json:"label"`
	BaseURL string       `json:"baseUrl"`
	Token   SecretStatus `json:"token"`
}

type View struct {
	QMediaSync QMediaSyncView `json:"qmediaSync"`
	Emby       EmbyView       `json:"emby"`
	Drive115   Drive115View   `json:"drive115"`
	TMDB       TMDBView       `json:"tmdb"`
	Workflow   Workflow       `json:"workflow"`
	Sources    []SourceView   `json:"sources"`
}

var sourceLabels = map[string]string{
	"dian": "点点", "framehdr": "帧影", "gimy": "Gimy", "guanying": "观影",
	"hdhive": "HDHive", "juying": "聚影", "mikan": "蜜柑", "sidhub": "Sidhub",
}

func FromConfig(value config.Config) Values {
	return Values{
		QMediaSync: value.QMediaSync,
		Emby:       value.Emby,
		Drive115:   value.Drive115,
		TMDB:       value.TMDB,
		Workflow:   value.Workflow,
		Sources:    append([]config.SearchSource(nil), value.Sources...),
	}
}
