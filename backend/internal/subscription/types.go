package subscription

import "time"

type Preferences struct {
	Resolutions      []string `json:"resolutions"`
	VideoCodecs      []string `json:"videoCodecs"`
	DynamicRanges    []string `json:"dynamicRanges"`
	AudioContains    []string `json:"audioContains"`
	PreferredSources []string `json:"preferredSources"`
	MinSizeBytes     int64    `json:"minSizeBytes"`
	MaxSizeBytes     int64    `json:"maxSizeBytes"`
	AllowUnknownSize bool     `json:"allowUnknownSize"`
	PreferSmaller    bool     `json:"preferSmaller"`
}

type CreateInput struct {
	TMDBID          string      `json:"tmdbId"`
	Title           string      `json:"title"`
	OriginalTitle   string      `json:"originalTitle"`
	Year            int         `json:"year"`
	MediaType       string      `json:"mediaType"`
	Season          int         `json:"season"`
	Policy          string      `json:"policy"`
	Enabled         bool        `json:"enabled"`
	IntervalMinutes int         `json:"intervalMinutes"`
	SourceIDs       []string    `json:"sourceIds"`
	Preferences     Preferences `json:"preferences"`
}

type UpdateInput struct {
	Title           string      `json:"title"`
	OriginalTitle   string      `json:"originalTitle"`
	Year            int         `json:"year"`
	Policy          string      `json:"policy"`
	Enabled         bool        `json:"enabled"`
	IntervalMinutes int         `json:"intervalMinutes"`
	SourceIDs       []string    `json:"sourceIds"`
	Preferences     Preferences `json:"preferences"`
}

type Subscription struct {
	ID              string      `json:"id"`
	TMDBID          string      `json:"tmdbId"`
	Title           string      `json:"title"`
	OriginalTitle   string      `json:"originalTitle,omitempty"`
	Year            int         `json:"year"`
	MediaType       string      `json:"mediaType"`
	Season          int         `json:"season,omitempty"`
	LastEpisode     int         `json:"lastEpisode,omitempty"`
	Policy          string      `json:"policy"`
	Enabled         bool        `json:"enabled"`
	IntervalMinutes int         `json:"intervalMinutes"`
	SourceIDs       []string    `json:"sourceIds"`
	Preferences     Preferences `json:"preferences"`
	NextRunAt       time.Time   `json:"nextRunAt"`
	LastRunAt       *time.Time  `json:"lastRunAt,omitempty"`
	CreatedAt       time.Time   `json:"createdAt"`
	UpdatedAt       time.Time   `json:"updatedAt"`
}

type Run struct {
	ID             string     `json:"id"`
	SubscriptionID string     `json:"subscriptionId"`
	TriggerType    string     `json:"triggerType"`
	State          string     `json:"state"`
	SourceID       string     `json:"sourceId,omitempty"`
	TransferJobID  string     `json:"transferJobId,omitempty"`
	ErrorCode      string     `json:"errorCode,omitempty"`
	Message        string     `json:"message,omitempty"`
	Retryable      bool       `json:"retryable"`
	StartedAt      time.Time  `json:"startedAt"`
	FinishedAt     *time.Time `json:"finishedAt,omitempty"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type Backup struct {
	Version       int           `json:"version"`
	ExportedAt    time.Time     `json:"exportedAt"`
	Subscriptions []CreateInput `json:"subscriptions"`
}

type ImportResult struct {
	Created int `json:"created"`
	Skipped int `json:"skipped"`
}

type BatchEnabledInput struct {
	IDs     []string `json:"ids"`
	Enabled bool     `json:"enabled"`
}
