package subscription

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"media-hub/backend/internal/emby"
	"media-hub/backend/internal/search"
	"media-hub/backend/internal/store"
	"media-hub/backend/internal/workflow"
)

var (
	ErrInvalidSubscription = errors.New("invalid subscription")
	ErrUnavailable         = errors.New("subscription service unavailable")
)

var (
	tmdbIDPattern   = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)
	sourceIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,49}$`)
)

const (
	minIntervalMinutes = 15
	maxIntervalMinutes = 7 * 24 * 60
)

type SearchProvider interface {
	Search(context.Context, string) search.Response
}

type TransferWorkflow interface {
	SelectionToken(search.Candidate) string
	Enqueue(context.Context, int64, string, string) (workflow.Job, bool, error)
	Get(context.Context, int64, string) (workflow.JobDetail, error)
}

type LibraryChecker interface {
	FindPlayableItem(context.Context, string, string, int, string, int, int, int) (emby.Item, bool, error)
}

type Service struct {
	store    *store.Store
	search   SearchProvider
	workflow TransferWorkflow
	library  LibraryChecker
	now      func() time.Time
	wake     chan struct{}
	done     chan struct{}
}

func NewService(dataStore *store.Store, searchProvider SearchProvider, transferWorkflow TransferWorkflow, library LibraryChecker) *Service {
	return &Service{
		store: dataStore, search: searchProvider, workflow: transferWorkflow, library: library,
		now: time.Now, wake: make(chan struct{}, 1), done: make(chan struct{}),
	}
}

func (s *Service) Create(ctx context.Context, userID int64, input CreateInput) (Subscription, error) {
	if s == nil || s.store == nil {
		return Subscription{}, ErrUnavailable
	}
	normalized, sources, preferences, err := normalizeCreate(input)
	if err != nil {
		return Subscription{}, err
	}
	id, err := randomID()
	if err != nil {
		return Subscription{}, err
	}
	now := s.now().UTC()
	nextRunAt := int64(0)
	if normalized.Enabled {
		nextRunAt = now.Unix()
	}
	stored, err := s.store.CreateSubscription(ctx, store.Subscription{
		ID: id, UserID: userID, TMDBID: normalized.TMDBID, Title: normalized.Title,
		OriginalTitle: normalized.OriginalTitle, Year: normalized.Year, MediaType: normalized.MediaType,
		Season: normalized.Season, Policy: normalized.Policy, Enabled: normalized.Enabled,
		IntervalMinutes: normalized.IntervalMinutes, SourceIDsJSON: sources, PreferencesJSON: preferences,
		NextRunAt: nextRunAt, CreatedAt: now.Unix(), UpdatedAt: now.Unix(),
	})
	if err != nil {
		return Subscription{}, err
	}
	if stored.Enabled {
		s.notify()
	}
	return publicSubscription(stored)
}

func (s *Service) Update(ctx context.Context, userID int64, id string, input UpdateInput) (Subscription, error) {
	if s == nil || s.store == nil {
		return Subscription{}, ErrUnavailable
	}
	stored, err := s.store.Subscription(ctx, userID, id)
	if err != nil {
		return Subscription{}, err
	}
	normalized, sources, preferences, err := normalizeUpdate(input)
	if err != nil {
		return Subscription{}, err
	}
	wasEnabled := stored.Enabled
	now := s.now().UTC()
	stored.Title = normalized.Title
	stored.OriginalTitle = normalized.OriginalTitle
	stored.Year = normalized.Year
	stored.Policy = normalized.Policy
	stored.Enabled = normalized.Enabled
	stored.IntervalMinutes = normalized.IntervalMinutes
	stored.SourceIDsJSON = sources
	stored.PreferencesJSON = preferences
	stored.UpdatedAt = now.Unix()
	if stored.Enabled && !wasEnabled {
		stored.NextRunAt = now.Unix()
	} else if !stored.Enabled {
		stored.NextRunAt = 0
	}
	stored, err = s.store.UpdateSubscription(ctx, stored)
	if err != nil {
		return Subscription{}, err
	}
	if stored.Enabled {
		s.notify()
	}
	return publicSubscription(stored)
}

func (s *Service) Get(ctx context.Context, userID int64, id string) (Subscription, error) {
	if s == nil || s.store == nil {
		return Subscription{}, ErrUnavailable
	}
	stored, err := s.store.Subscription(ctx, userID, id)
	if err != nil {
		return Subscription{}, err
	}
	return publicSubscription(stored)
}

func (s *Service) List(ctx context.Context, userID int64) ([]Subscription, error) {
	if s == nil || s.store == nil {
		return nil, ErrUnavailable
	}
	stored, err := s.store.ListSubscriptions(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]Subscription, 0, len(stored))
	for _, item := range stored {
		value, err := publicSubscription(item)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func (s *Service) Export(ctx context.Context, userID int64) (Backup, error) {
	items, err := s.List(ctx, userID)
	if err != nil {
		return Backup{}, err
	}
	values := make([]CreateInput, 0, len(items))
	for _, item := range items {
		values = append(values, CreateInput{
			TMDBID: item.TMDBID, Title: item.Title, OriginalTitle: item.OriginalTitle, Year: item.Year,
			MediaType: item.MediaType, Season: item.Season, Policy: item.Policy, Enabled: item.Enabled,
			IntervalMinutes: item.IntervalMinutes, SourceIDs: item.SourceIDs, Preferences: item.Preferences,
		})
	}
	return Backup{Version: 1, ExportedAt: s.now().UTC(), Subscriptions: values}, nil
}

func (s *Service) Import(ctx context.Context, userID int64, backup Backup) (ImportResult, error) {
	if s == nil || s.store == nil || backup.Version != 1 || len(backup.Subscriptions) > 1000 {
		return ImportResult{}, ErrInvalidSubscription
	}
	now := s.now().UTC()
	items := make([]store.Subscription, 0, len(backup.Subscriptions))
	for _, input := range backup.Subscriptions {
		normalized, sources, preferences, err := normalizeCreate(input)
		if err != nil {
			return ImportResult{}, err
		}
		id, err := randomID()
		if err != nil {
			return ImportResult{}, err
		}
		nextRunAt := int64(0)
		if normalized.Enabled {
			nextRunAt = now.Unix()
		}
		items = append(items, store.Subscription{
			ID: id, UserID: userID, TMDBID: normalized.TMDBID, Title: normalized.Title,
			OriginalTitle: normalized.OriginalTitle, Year: normalized.Year, MediaType: normalized.MediaType,
			Season: normalized.Season, Policy: normalized.Policy, Enabled: normalized.Enabled,
			IntervalMinutes: normalized.IntervalMinutes, SourceIDsJSON: sources, PreferencesJSON: preferences,
			NextRunAt: nextRunAt, CreatedAt: now.Unix(), UpdatedAt: now.Unix(),
		})
	}
	created, err := s.store.ImportSubscriptions(ctx, items)
	if err != nil {
		return ImportResult{}, err
	}
	if created > 0 {
		s.notify()
	}
	return ImportResult{Created: created, Skipped: len(items) - created}, nil
}

func (s *Service) SetBatchEnabled(ctx context.Context, userID int64, input BatchEnabledInput) ([]Subscription, error) {
	if s == nil || s.store == nil || len(input.IDs) == 0 || len(input.IDs) > 1000 {
		return nil, ErrInvalidSubscription
	}
	seen := make(map[string]struct{}, len(input.IDs))
	ids := make([]string, 0, len(input.IDs))
	for _, raw := range input.IDs {
		id := strings.TrimSpace(raw)
		if id == "" || len(id) > 100 {
			return nil, ErrInvalidSubscription
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	nextRunAt := int64(0)
	if input.Enabled {
		nextRunAt = s.now().UTC().Unix()
	}
	if err := s.store.SetSubscriptionsEnabled(ctx, userID, ids, input.Enabled, nextRunAt, s.now()); err != nil {
		return nil, err
	}
	if input.Enabled {
		s.notify()
	}
	return s.List(ctx, userID)
}

func (s *Service) Delete(ctx context.Context, userID int64, id string) error {
	if s == nil || s.store == nil {
		return ErrUnavailable
	}
	return s.store.DeleteSubscription(ctx, userID, id)
}

func (s *Service) SetEnabled(ctx context.Context, userID int64, id string, enabled bool) (Subscription, error) {
	if s == nil || s.store == nil {
		return Subscription{}, ErrUnavailable
	}
	nextRunAt := int64(0)
	if enabled {
		nextRunAt = s.now().UTC().Unix()
	}
	stored, err := s.store.SetSubscriptionEnabled(ctx, userID, id, enabled, nextRunAt, s.now())
	if err != nil {
		return Subscription{}, err
	}
	if enabled {
		s.notify()
	}
	return publicSubscription(stored)
}

func (s *Service) RunNow(ctx context.Context, userID int64, id string) (Run, error) {
	if s == nil || s.store == nil {
		return Run{}, ErrUnavailable
	}
	runID, err := randomID()
	if err != nil {
		return Run{}, err
	}
	stored, err := s.store.CreateManualSubscriptionRun(ctx, userID, id, runID, s.now())
	if err != nil {
		return Run{}, err
	}
	s.notify()
	return publicRun(stored), nil
}

func (s *Service) Runs(ctx context.Context, userID int64, id string, limit int) ([]Run, error) {
	if s == nil || s.store == nil {
		return nil, ErrUnavailable
	}
	stored, err := s.store.SubscriptionRuns(ctx, userID, id, limit)
	if err != nil {
		return nil, err
	}
	result := make([]Run, 0, len(stored))
	for _, item := range stored {
		result = append(result, publicRun(item))
	}
	return result, nil
}

func normalizeCreate(input CreateInput) (CreateInput, string, string, error) {
	input.TMDBID = strings.TrimSpace(input.TMDBID)
	input.Title = strings.TrimSpace(input.Title)
	input.OriginalTitle = strings.TrimSpace(input.OriginalTitle)
	input.MediaType = strings.TrimSpace(input.MediaType)
	input.Policy = strings.TrimSpace(input.Policy)
	if !validIdentity(input.TMDBID, input.Title, input.OriginalTitle, input.Year, input.MediaType, input.Season) ||
		(input.Policy != "once" && input.Policy != "upgrade") ||
		input.IntervalMinutes < minIntervalMinutes || input.IntervalMinutes > maxIntervalMinutes {
		return CreateInput{}, "", "", ErrInvalidSubscription
	}
	sources, err := normalizeSourceIDs(input.SourceIDs)
	if err != nil {
		return CreateInput{}, "", "", err
	}
	preferences, err := normalizePreferences(input.Preferences)
	if err != nil {
		return CreateInput{}, "", "", err
	}
	input.SourceIDs = sources
	input.Preferences = preferences
	sourceJSON, _ := json.Marshal(sources)
	preferenceJSON, _ := json.Marshal(preferences)
	return input, string(sourceJSON), string(preferenceJSON), nil
}

func normalizeUpdate(input UpdateInput) (UpdateInput, string, string, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.OriginalTitle = strings.TrimSpace(input.OriginalTitle)
	input.Policy = strings.TrimSpace(input.Policy)
	if input.Title == "" || len([]rune(input.Title)) > 300 || len([]rune(input.OriginalTitle)) > 300 ||
		input.Year < 0 || input.Year > 2100 ||
		(input.Policy != "once" && input.Policy != "upgrade") ||
		input.IntervalMinutes < minIntervalMinutes || input.IntervalMinutes > maxIntervalMinutes {
		return UpdateInput{}, "", "", ErrInvalidSubscription
	}
	sources, err := normalizeSourceIDs(input.SourceIDs)
	if err != nil {
		return UpdateInput{}, "", "", err
	}
	preferences, err := normalizePreferences(input.Preferences)
	if err != nil {
		return UpdateInput{}, "", "", err
	}
	input.SourceIDs = sources
	input.Preferences = preferences
	sourceJSON, _ := json.Marshal(sources)
	preferenceJSON, _ := json.Marshal(preferences)
	return input, string(sourceJSON), string(preferenceJSON), nil
}

func validIdentity(tmdbID, title, originalTitle string, year int, mediaType string, season int) bool {
	if !tmdbIDPattern.MatchString(tmdbID) || title == "" || len([]rune(title)) > 300 || len([]rune(originalTitle)) > 300 {
		return false
	}
	if year < 0 || year > 2100 || season < 0 || season > 100 {
		return false
	}
	if mediaType != "movie" && mediaType != "series" {
		return false
	}
	return mediaType != "movie" || season == 0
}

func normalizeSourceIDs(values []string) ([]string, error) {
	if len(values) > 16 {
		return nil, ErrInvalidSubscription
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if !sourceIDPattern.MatchString(value) {
			return nil, ErrInvalidSubscription
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func normalizePreferences(input Preferences) (Preferences, error) {
	var err error
	if input.Resolutions, err = normalizeValues(input.Resolutions, 16); err != nil {
		return Preferences{}, err
	}
	if input.VideoCodecs, err = normalizeValues(input.VideoCodecs, 16); err != nil {
		return Preferences{}, err
	}
	if input.DynamicRanges, err = normalizeValues(input.DynamicRanges, 16); err != nil {
		return Preferences{}, err
	}
	if input.AudioContains, err = normalizeValues(input.AudioContains, 16); err != nil {
		return Preferences{}, err
	}
	if input.PreferredSources, err = normalizeSourceIDs(input.PreferredSources); err != nil {
		return Preferences{}, err
	}
	if input.MinSizeBytes < 0 || input.MaxSizeBytes < 0 ||
		(input.MaxSizeBytes > 0 && input.MinSizeBytes > input.MaxSizeBytes) {
		return Preferences{}, ErrInvalidSubscription
	}
	return input, nil
}

func normalizeValues(values []string, limit int) ([]string, error) {
	if len(values) > limit {
		return nil, ErrInvalidSubscription
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" || len([]rune(value)) > 80 {
			return nil, ErrInvalidSubscription
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func publicSubscription(item store.Subscription) (Subscription, error) {
	var sources []string
	var preferences Preferences
	if err := json.Unmarshal([]byte(item.SourceIDsJSON), &sources); err != nil {
		return Subscription{}, err
	}
	if err := json.Unmarshal([]byte(item.PreferencesJSON), &preferences); err != nil {
		return Subscription{}, err
	}
	result := Subscription{
		ID: item.ID, TMDBID: item.TMDBID, Title: item.Title, OriginalTitle: item.OriginalTitle,
		Year: item.Year, MediaType: item.MediaType, Season: item.Season, LastEpisode: item.LastEpisode, Policy: item.Policy,
		Enabled: item.Enabled, IntervalMinutes: item.IntervalMinutes, SourceIDs: sources,
		Preferences: preferences, NextRunAt: time.Unix(item.NextRunAt, 0).UTC(),
		CreatedAt: time.Unix(item.CreatedAt, 0).UTC(), UpdatedAt: time.Unix(item.UpdatedAt, 0).UTC(),
	}
	if item.LastRunAt > 0 {
		last := time.Unix(item.LastRunAt, 0).UTC()
		result.LastRunAt = &last
	}
	return result, nil
}

func publicRun(item store.SubscriptionRun) Run {
	result := Run{
		ID: item.ID, SubscriptionID: item.SubscriptionID, TriggerType: item.TriggerType,
		State: item.State, SourceID: item.SourceID, TransferJobID: item.TransferJobID,
		ErrorCode: item.ErrorCode, Message: item.Message, Retryable: item.Retryable,
		StartedAt: time.Unix(item.StartedAt, 0).UTC(), UpdatedAt: time.Unix(item.UpdatedAt, 0).UTC(),
	}
	if item.FinishedAt > 0 {
		finished := time.Unix(item.FinishedAt, 0).UTC()
		result.FinishedAt = &finished
	}
	return result
}

func randomID() (string, error) {
	value := make([]byte, 18)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func deterministicIdempotencyKey(runID string) string {
	return "subscription_" + runID
}

func seasonLabel(season int) string {
	if season <= 0 {
		return ""
	}
	return " S" + strconv.Itoa(season)
}

func (s *Service) notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}
