package moviepilot

import (
	"context"

	"media-hub/backend/internal/search"
)

type Source struct {
	client *Client
}

func NewSource(client *Client) *Source {
	return &Source{client: client}
}

func (s *Source) ID() string    { return SourceID }
func (s *Source) Label() string { return SourceLabel }

func (s *Source) Search(ctx context.Context, query string) ([]search.Candidate, error) {
	if s == nil || s.client == nil {
		return nil, ErrNotConfigured
	}
	return s.client.Search(ctx, query)
}

func (s *Source) StartDownload(ctx context.Context, request search.DownloadRequest) error {
	if s == nil || s.client == nil {
		return ErrNotConfigured
	}
	return s.client.StartDownload(ctx, request)
}
