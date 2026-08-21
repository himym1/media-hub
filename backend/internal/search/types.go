package search

import "context"

type Source interface {
	ID() string
	Label() string
	Search(context.Context, string) ([]Candidate, error)
}

type TransferSource interface {
	Source
	StartTransfer(context.Context, TransferRequest) (TransferResult, error)
	TransferStatus(context.Context, int64, string) (TransferResult, error)
}

type CheckInSource interface {
	Source
	CheckIn(context.Context) (CheckInResult, error)
}

type CheckInResult struct {
	State   string
	Message string
}

type IdentityResolver interface {
	Resolve(context.Context, string) ([]Identity, error)
}

type Identity struct {
	TMDBID        string
	Title         string
	OriginalTitle string
	Year          int
	MediaType     string
	PosterURL     string
}

type TransferRequest struct {
	UserID         int64
	Reference      string
	DestinationID  string
	IdempotencyKey string
}

type TransferResult struct {
	OperationID string
	Status      string
	FileID      string
	Path        string
	IsFile      bool
}

type Candidate struct {
	ID               string       `json:"id"`
	Title            string       `json:"title"`
	Year             int          `json:"year"`
	Season           int          `json:"season,omitempty"`
	EpisodeStart     int          `json:"episodeStart,omitempty"`
	EpisodeEnd       int          `json:"episodeEnd,omitempty"`
	MediaType        string       `json:"mediaType"`
	TMDBID           string       `json:"tmdbId,omitempty"`
	Source           string       `json:"source"`
	Provider         string       `json:"provider,omitempty"`
	PosterURL        string       `json:"posterUrl,omitempty"`
	Release          ReleaseFacts `json:"release"`
	TransferState    string       `json:"transferState"`
	SourceID         string       `json:"sourceId"`
	SourceRef        string       `json:"-"`
	ReleaseTitle     string       `json:"-"`
	IdentityVerified bool         `json:"-"`
	Revision         uint64       `json:"-"`
}

type ReleaseFacts struct {
	Resolution   string `json:"resolution"`
	VideoCodec   string `json:"videoCodec"`
	DynamicRange string `json:"dynamicRange,omitempty"`
	Audio        string `json:"audio,omitempty"`
	SizeBytes    int64  `json:"sizeBytes"`
}

type Response struct {
	Query        string        `json:"query"`
	Partial      bool          `json:"partial"`
	Results      []Candidate   `json:"results"`
	SourceErrors []SourceError `json:"sourceErrors"`
}

type SourceError struct {
	Source    string `json:"source"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type Failure struct {
	Code      string
	Message   string
	Retryable bool
}

func (failure Failure) Error() string {
	return failure.Code
}
