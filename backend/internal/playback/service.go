package playback

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidRequest      = errors.New("invalid playback request")
	ErrNotFound            = errors.New("playable media not found")
	ErrUnavailable         = errors.New("playback is unavailable")
	ErrSourceNotConfigured = errors.New("playback source is not configured")
	ErrSourceUnauthorized  = errors.New("playback source rejected authentication")
)

const (
	PlayerUserAgent = "Mozilla/5.0 (Linux; Android 8.0; MediaHubPlayer) AppleWebKit/537.36 Chrome/122.0 Mobile Safari/537.36"
	sessionTTL      = 24 * time.Hour
)

type Drive115Target struct {
	ParentID string
	FileID   string
}

type EmbyItemTarget struct {
	ItemID string
}

type Drive115Resolver interface {
	ResolveDrive115(context.Context, Drive115Target, string) (SourceMedia, error)
}

type EmbyResolver interface {
	ResolveEmbyItem(context.Context, EmbyItemTarget, string) (SourceMedia, error)
}

type SessionEventType string

const (
	SessionStarted  SessionEventType = "started"
	SessionProgress SessionEventType = "progress"
	SessionStopped  SessionEventType = "stopped"
)

type SessionEvent struct {
	Type       SessionEventType
	PositionMS int64
	Paused     bool
}

type SessionReporter interface {
	ReportPlayback(context.Context, string, SessionEvent) error
}

type SourceSession struct {
	Reporter  SessionReporter
	Reference string
}

type SourceMedia struct {
	URL             string
	Name            string
	ExpiresAt       *time.Time
	Session         *SourceSession
	StartPositionMS int64
}

type Descriptor struct {
	StreamURL       string     `json:"streamUrl"`
	UserAgent       string     `json:"userAgent"`
	Title           string     `json:"title"`
	ExpiresAt       *time.Time `json:"expiresAt,omitempty"`
	SessionID       string     `json:"sessionId,omitempty"`
	StartPositionMS int64      `json:"startPositionMs,omitempty"`
}

type playbackSession struct {
	UserID    int64
	Source    SourceSession
	ExpiresAt time.Time
	mutex     sync.Mutex
	stopped   bool
}

type Service struct {
	drive115 Drive115Resolver
	emby     EmbyResolver
	now      func() time.Time
	mutex    sync.Mutex
	sessions map[string]*playbackSession
}

func NewService(drive115 Drive115Resolver, emby EmbyResolver) *Service {
	return &Service{drive115: drive115, emby: emby, now: time.Now, sessions: make(map[string]*playbackSession)}
}

func (s *Service) CreateDrive115(ctx context.Context, target Drive115Target) (Descriptor, error) {
	target.ParentID = strings.TrimSpace(target.ParentID)
	target.FileID = strings.TrimSpace(target.FileID)
	if s == nil || s.drive115 == nil {
		return Descriptor{}, ErrUnavailable
	}
	if !numericID(target.ParentID) || !numericID(target.FileID) {
		return Descriptor{}, ErrInvalidRequest
	}
	media, err := s.drive115.ResolveDrive115(ctx, target, PlayerUserAgent)
	if err != nil {
		return Descriptor{}, err
	}
	if !isVideoName(media.Name) {
		return Descriptor{}, ErrNotFound
	}
	return descriptor(media)
}

func (s *Service) CreateEmbyItem(ctx context.Context, userID int64, target EmbyItemTarget) (Descriptor, error) {
	target.ItemID = strings.TrimSpace(target.ItemID)
	if s == nil || s.emby == nil {
		return Descriptor{}, ErrUnavailable
	}
	if userID < 1 || !opaqueID(target.ItemID) {
		return Descriptor{}, ErrInvalidRequest
	}
	media, err := s.emby.ResolveEmbyItem(ctx, target, PlayerUserAgent)
	if err != nil {
		return Descriptor{}, err
	}
	value, err := descriptor(media)
	if err != nil {
		return Descriptor{}, err
	}
	if media.Session != nil && media.Session.Reporter != nil && media.Session.Reference != "" {
		value.SessionID, err = s.createSession(userID, *media.Session)
		if err != nil {
			return Descriptor{}, ErrUnavailable
		}
	}
	return value, nil
}

func (s *Service) Report(ctx context.Context, userID int64, sessionID string, event SessionEvent) error {
	if s == nil || userID < 1 || !validSessionID(sessionID) || !validSessionEvent(event) {
		return ErrInvalidRequest
	}
	s.mutex.Lock()
	session, ok := s.sessions[sessionID]
	if ok && !s.now().Before(session.ExpiresAt) {
		delete(s.sessions, sessionID)
		ok = false
	}
	if ok && session.UserID != userID {
		ok = false
	}
	s.mutex.Unlock()
	if !ok {
		return ErrNotFound
	}
	session.mutex.Lock()
	defer session.mutex.Unlock()
	if session.stopped {
		return ErrNotFound
	}
	if err := session.Source.Reporter.ReportPlayback(ctx, session.Source.Reference, event); err != nil {
		return err
	}
	if event.Type == SessionStopped {
		session.stopped = true
		s.mutex.Lock()
		delete(s.sessions, sessionID)
		s.mutex.Unlock()
	}
	return nil
}

func (s *Service) createSession(userID int64, source SourceSession) (string, error) {
	value := make([]byte, 24)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	id := hex.EncodeToString(value)
	s.mutex.Lock()
	now := s.now()
	for existingID, session := range s.sessions {
		if !now.Before(session.ExpiresAt) {
			delete(s.sessions, existingID)
		}
	}
	s.sessions[id] = &playbackSession{UserID: userID, Source: source, ExpiresAt: now.Add(sessionTTL)}
	s.mutex.Unlock()
	return id, nil
}

func descriptor(media SourceMedia) (Descriptor, error) {
	parsed, err := url.Parse(strings.TrimSpace(media.URL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return Descriptor{}, ErrUnavailable
	}
	name := strings.TrimSpace(media.Name)
	if name == "" || len([]rune(name)) > 255 {
		return Descriptor{}, ErrNotFound
	}
	return Descriptor{
		StreamURL: parsed.String(), UserAgent: PlayerUserAgent, Title: name, ExpiresAt: media.ExpiresAt,
		StartPositionMS: media.StartPositionMS,
	}, nil
}

func validSessionEvent(event SessionEvent) bool {
	if event.Type != SessionStarted && event.Type != SessionProgress && event.Type != SessionStopped {
		return false
	}
	return event.PositionMS >= 0 && event.PositionMS <= int64((7*24*time.Hour)/time.Millisecond)
}

func validSessionID(value string) bool {
	if len(value) != 48 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
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

func opaqueID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

func isVideoName(name string) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(name))) {
	case ".mp4", ".mkv", ".m4v", ".mov", ".webm", ".avi", ".ts", ".m2ts":
		return true
	default:
		return false
	}
}
