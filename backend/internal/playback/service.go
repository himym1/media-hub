package playback

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrInvalidRequest      = errors.New("invalid playback request")
	ErrNotFound            = errors.New("playable media not found")
	ErrUnavailable         = errors.New("playback is unavailable")
	ErrSourceNotConfigured = errors.New("playback source is not configured")
	ErrSourceUnauthorized  = errors.New("playback source rejected authentication")
)

const PlayerUserAgent = "Mozilla/5.0 (Linux; Android 8.0; MediaHubPlayer) AppleWebKit/537.36 Chrome/122.0 Mobile Safari/537.36"

type Source interface {
	Resolve(context.Context, string, string, string) (SourceMedia, error)
}

type SourceMedia struct {
	URL       string
	Name      string
	ExpiresAt *time.Time
}

type Descriptor struct {
	StreamURL string     `json:"streamUrl"`
	UserAgent string     `json:"userAgent"`
	Title     string     `json:"title"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

type Service struct {
	source Source
}

func NewService(source Source) *Service {
	return &Service{source: source}
}

func (s *Service) Create(ctx context.Context, parentID, fileID string) (Descriptor, error) {
	parentID = strings.TrimSpace(parentID)
	fileID = strings.TrimSpace(fileID)
	if s == nil || s.source == nil {
		return Descriptor{}, ErrUnavailable
	}
	if !numericID(parentID) || !numericID(fileID) {
		return Descriptor{}, ErrInvalidRequest
	}
	media, err := s.source.Resolve(ctx, parentID, fileID, PlayerUserAgent)
	if err != nil {
		return Descriptor{}, err
	}
	parsed, err := url.Parse(media.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return Descriptor{}, ErrUnavailable
	}
	name := strings.TrimSpace(media.Name)
	if name == "" || !isVideoName(name) {
		return Descriptor{}, ErrNotFound
	}
	return Descriptor{
		StreamURL: parsed.String(), UserAgent: PlayerUserAgent, Title: name, ExpiresAt: media.ExpiresAt,
	}, nil
}

func numericID(value string) bool {
	if value == "" || len(value) > 32 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func isVideoName(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp4", ".mkv", ".m4v", ".mov", ".webm", ".avi", ".ts", ".m2ts":
		return true
	default:
		return false
	}
}
