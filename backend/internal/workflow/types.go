package workflow

import "time"

type Job struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Year         int       `json:"year"`
	Season       int       `json:"season,omitempty"`
	EpisodeStart int       `json:"episodeStart,omitempty"`
	EpisodeEnd   int       `json:"episodeEnd,omitempty"`
	MediaType    string    `json:"mediaType"`
	TMDBID       string    `json:"tmdbId"`
	Source       string    `json:"source"`
	State        string    `json:"state"`
	ErrorCode    string    `json:"errorCode,omitempty"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
	Retryable    bool      `json:"retryable"`
	Archived     bool      `json:"archived"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type Event struct {
	ID        int64     `json:"id"`
	State     string    `json:"state"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type JobDetail struct {
	Job
	Events []Event `json:"events"`
}

type Notification struct {
	ID        string    `json:"id"`
	JobID     string    `json:"jobId"`
	EventType string    `json:"eventType"`
	JobState  string    `json:"jobState"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	Attempts  int       `json:"attempts"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
