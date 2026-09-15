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
	embyTicketTTL   = 50 * time.Minute
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
	ResolvePickCode(context.Context, string, string, string) (SourceMedia, error)
}

type LocalRef struct {
	ItemID        string
	MediaSourceID string
	Container     string
}

type EmbyResolver interface {
	ResolveEmbyItem(context.Context, EmbyItemTarget, string) (SourceMedia, error)
	LocalStreamURL(LocalRef) (string, error)
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
	PickCode        string
	ExpiresAt       *time.Time
	Session         *SourceSession
	StartPositionMS int64
	Local           *LocalRef
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

type embyTicket struct {
	ref       LocalRef
	expiresAt time.Time
}

type Service struct {
	drive115   Drive115Resolver
	emby       EmbyResolver
	now        func() time.Time
	mutex      sync.Mutex
	sessions   map[string]*playbackSession
	publicBase string
	tickets    map[string]embyTicket
}

func NewService(drive115 Drive115Resolver, emby EmbyResolver) *Service {
	return &Service{drive115: drive115, emby: emby, now: time.Now, sessions: make(map[string]*playbackSession), tickets: map[string]embyTicket{}}
}

func (s *Service) ConfigurePublicBase(baseURL string) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		baseURL = ""
	} else {
		baseURL = strings.TrimRight(parsed.String(), "/")
	}
	if s == nil {
		return
	}
	s.mutex.Lock()
	s.publicBase = baseURL
	s.mutex.Unlock()
}

func ResolvePlaybackUserAgent(value string) (string, error) {
	userAgent := strings.TrimSpace(value)
	if userAgent == "" {
		return PlayerUserAgent, nil
	}
	if len(userAgent) > 512 {
		return "", ErrInvalidRequest
	}
	for _, character := range userAgent {
		if character < 32 || character == 127 {
			return "", ErrInvalidRequest
		}
	}
	return userAgent, nil
}

func (s *Service) CreateDrive115(ctx context.Context, target Drive115Target, playbackUserAgent string) (Descriptor, error) {
	target.ParentID = strings.TrimSpace(target.ParentID)
	target.FileID = strings.TrimSpace(target.FileID)
	userAgent, err := ResolvePlaybackUserAgent(playbackUserAgent)
	if err != nil {
		return Descriptor{}, err
	}
	if s == nil || s.drive115 == nil {
		return Descriptor{}, ErrUnavailable
	}
	if !numericID(target.ParentID) || !numericID(target.FileID) {
		return Descriptor{}, ErrInvalidRequest
	}
	media, err := s.drive115.ResolveDrive115(ctx, target, userAgent)
	if err != nil {
		return Descriptor{}, err
	}
	if !isVideoName(media.Name) {
		return Descriptor{}, ErrNotFound
	}
	return descriptor(media, userAgent)
}

func (s *Service) CreateEmbyItem(ctx context.Context, userID int64, target EmbyItemTarget, playbackUserAgent string) (Descriptor, error) {
	target.ItemID = strings.TrimSpace(target.ItemID)
	userAgent, err := ResolvePlaybackUserAgent(playbackUserAgent)
	if err != nil {
		return Descriptor{}, err
	}
	if s == nil || s.emby == nil {
		return Descriptor{}, ErrUnavailable
	}
	if userID < 1 || !opaqueID(target.ItemID) {
		return Descriptor{}, ErrInvalidRequest
	}
	media, err := s.emby.ResolveEmbyItem(ctx, target, userAgent)
	if err != nil {
		return Descriptor{}, err
	}
	if media.Local != nil {
		streamURL, expiresAt, issueErr := s.issueEmbyTicket(*media.Local, media.Name)
		if issueErr != nil {
			return Descriptor{}, issueErr
		}
		media.URL = streamURL
		media.ExpiresAt = expiresAt
	} else if code := validPickCode(media.PickCode); code != "" {
		if s.drive115 == nil {
			return Descriptor{}, ErrUnavailable
		}
		resolved, resolveErr := s.drive115.ResolvePickCode(ctx, code, media.Name, userAgent)
		if resolveErr != nil {
			if strings.TrimSpace(media.URL) == "" {
				return Descriptor{}, resolveErr
			}
		} else {
			media.URL = resolved.URL
			if strings.TrimSpace(media.Name) == "" {
				media.Name = resolved.Name
			}
		}
	} else if strings.TrimSpace(media.URL) == "" {
		return Descriptor{}, ErrUnavailable
	}
	value, err := descriptor(media, userAgent)
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

func (s *Service) RedirectEmby(_ context.Context, ticket string) (string, error) {
	if s == nil || s.emby == nil {
		return "", ErrUnavailable
	}
	ticket = strings.TrimSpace(ticket)
	if !validSessionID(ticket) {
		return "", ErrInvalidRequest
	}
	s.mutex.Lock()
	item, ok := s.tickets[ticket]
	if ok && !s.now().Before(item.expiresAt) {
		delete(s.tickets, ticket)
		ok = false
	}
	s.mutex.Unlock()
	if !ok {
		return "", ErrNotFound
	}
	location, err := s.emby.LocalStreamURL(item.ref)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(strings.TrimSpace(location))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", ErrUnavailable
	}
	return parsed.String(), nil
}

func (s *Service) issueEmbyTicket(ref LocalRef, name string) (string, *time.Time, error) {
	s.mutex.Lock()
	base := s.publicBase
	s.mutex.Unlock()
	if base == "" {
		return "", nil, ErrUnavailable
	}
	if !opaqueID(ref.ItemID) || !opaqueID(ref.MediaSourceID) {
		return "", nil, ErrInvalidRequest
	}
	value := make([]byte, 24)
	if _, err := rand.Read(value); err != nil {
		return "", nil, ErrUnavailable
	}
	ticket := hex.EncodeToString(value)
	expires := s.now().Add(embyTicketTTL)
	s.mutex.Lock()
	now := s.now()
	for id, item := range s.tickets {
		if !now.Before(item.expiresAt) {
			delete(s.tickets, id)
		}
	}
	s.tickets[ticket] = embyTicket{ref: ref, expiresAt: expires}
	s.mutex.Unlock()
	streamURL, err := publicEmbyStreamURL(base, name, ref.Container, ticket)
	if err != nil {
		return "", nil, err
	}
	return streamURL, &expires, nil
}

func publicEmbyStreamURL(base, name, container, ticket string) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", ErrUnavailable
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/") + "/emby/url/video" + StreamExtension(container, name)
	query := parsed.Query()
	query.Set("ticket", ticket)
	parsed.RawQuery = query.Encode()
	parsed.Fragment = ""
	return parsed.String(), nil
}

func StreamExtension(container, name string) string {
	for _, value := range []string{container, filepath.Ext(name)} {
		ext := strings.ToLower(strings.Trim(strings.TrimSpace(value), "."))
		switch ext {
		case "mp4", "mkv", "m4v", "mov", "webm", "avi", "ts", "m2ts", "wmv":
			return "." + ext
		}
	}
	return ".mkv"
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

func descriptor(media SourceMedia, userAgent string) (Descriptor, error) {
	parsed, err := url.Parse(strings.TrimSpace(media.URL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return Descriptor{}, ErrUnavailable
	}
	name := strings.TrimSpace(media.Name)
	if name == "" || len([]rune(name)) > 255 {
		return Descriptor{}, ErrNotFound
	}
	return Descriptor{
		StreamURL: parsed.String(), UserAgent: userAgent, Title: name, ExpiresAt: media.ExpiresAt,
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

func validPickCode(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 4 || len(value) > 32 {
		return ""
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') {
			return ""
		}
	}
	return value
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
