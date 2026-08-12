package subxmigration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"media-hub/backend/internal/subscription"
	"media-hub/backend/internal/subx"
)

var (
	ErrUnavailable = errors.New("SubX migration is unavailable")
	ErrNoContent   = errors.New("SubX backup contains no importable subscriptions")
)

type SubXProvider interface {
	Configured() bool
	ExportContents(context.Context) (json.RawMessage, error)
}

type SubscriptionProvider interface {
	List(context.Context, int64) ([]subscription.Subscription, error)
	Import(context.Context, int64, subscription.Backup) (subscription.ImportResult, error)
}

type Readiness struct {
	CanStopSubX           bool     `json:"canStopSubX"`
	SubXConfigured        bool     `json:"subxConfigured"`
	FallbackSourceEnabled bool     `json:"fallbackSourceEnabled"`
	NativeSubscriptions   int      `json:"nativeSubscriptions"`
	DelegatedOperations   int      `json:"delegatedOperations"`
	DelegatedGroups       []string `json:"delegatedGroups"`
	Blockers              []string `json:"blockers"`
}

type ImportResult struct {
	Detected   int `json:"detected"`
	Importable int `json:"importable"`
	Rejected   int `json:"rejected"`
	Created    int `json:"created"`
	Skipped    int `json:"skipped"`
}

type Service struct {
	subx                  SubXProvider
	subscriptions         SubscriptionProvider
	now                   func() time.Time
	fallbackSourceEnabled bool
}

func NewService(subxProvider SubXProvider, subscriptions SubscriptionProvider, fallbackSourceEnabled ...bool) *Service {
	enabled := len(fallbackSourceEnabled) > 0 && fallbackSourceEnabled[0]
	return &Service{subx: subxProvider, subscriptions: subscriptions, now: time.Now, fallbackSourceEnabled: enabled}
}

func (s *Service) Readiness(ctx context.Context, userID int64) (Readiness, error) {
	if s == nil || s.subx == nil || s.subscriptions == nil {
		return Readiness{}, ErrUnavailable
	}
	items, err := s.subscriptions.List(ctx, userID)
	if err != nil {
		return Readiness{}, err
	}
	pendingCommands := 0
	if counter, ok := s.subx.(interface {
		PendingCommands(context.Context, int64) (int, error)
	}); ok {
		pendingCommands, err = counter.PendingCommands(ctx, userID)
		if err != nil {
			return Readiness{}, err
		}
	}
	result := Readiness{
		SubXConfigured: s.subx.Configured(), FallbackSourceEnabled: s.fallbackSourceEnabled, NativeSubscriptions: len(items),
		DelegatedGroups: []string{},
	}
	if s.fallbackSourceEnabled || pendingCommands > 0 {
		if s.fallbackSourceEnabled {
			result.DelegatedOperations++
			result.DelegatedGroups = append(result.DelegatedGroups, "migration-source")
			result.Blockers = append(result.Blockers, "SubX 迁移回退资源源仍处于启用状态")
		}
		if pendingCommands > 0 {
			result.DelegatedOperations += pendingCommands
			result.DelegatedGroups = append(result.DelegatedGroups, "pending-source-commands")
			result.Blockers = append(result.Blockers, fmt.Sprintf("仍有 %d 个 SubX source command 未得到确定终态", pendingCommands))
			if !s.fallbackSourceEnabled {
				result.Blockers = append(result.Blockers, "核对并重试遗留命令前需显式启用 MEDIA_HUB_SUBX_SOURCE_ENABLED")
			}
		}
		result.Blockers = append(result.Blockers, "停用前必须关闭回退能力并完成真实凭据验收")
	} else {
		result.CanStopSubX = true
		result.Blockers = []string{}
	}
	return result, nil
}

func (s *Service) ImportSubscriptions(ctx context.Context, userID int64) (ImportResult, error) {
	if s == nil || s.subx == nil || s.subscriptions == nil {
		return ImportResult{}, ErrUnavailable
	}
	raw, err := s.subx.ExportContents(ctx)
	if err != nil {
		return ImportResult{}, err
	}
	inputs, detected, rejected := extractSubscriptions(raw)
	if len(inputs) == 0 {
		return ImportResult{Detected: detected, Rejected: rejected}, ErrNoContent
	}
	imported, err := s.subscriptions.Import(ctx, userID, subscription.Backup{
		Version: 1, ExportedAt: s.now().UTC(), Subscriptions: inputs,
	})
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{
		Detected: detected, Importable: len(inputs), Rejected: rejected,
		Created: imported.Created, Skipped: imported.Skipped,
	}, nil
}

func (s *Service) BlockingCommands(ctx context.Context, userID int64) ([]subx.CommandJob, error) {
	provider, ok := s.subx.(interface {
		List(context.Context, int64, int) ([]subx.CommandJob, error)
	})
	if !ok {
		return nil, ErrUnavailable
	}
	items, err := provider.List(ctx, userID, 100)
	if err != nil {
		return nil, err
	}
	result := make([]subx.CommandJob, 0)
	for _, item := range items {
		if item.State == "queued" || item.State == "submitting" || item.State == "needs_attention" {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *Service) RetryBlockingCommand(ctx context.Context, userID int64, id, confirmation string) (subx.CommandJob, error) {
	if !s.fallbackSourceEnabled {
		return subx.CommandJob{}, ErrUnavailable
	}
	provider, ok := s.subx.(interface {
		Retry(context.Context, int64, string, string) (subx.CommandJob, error)
	})
	if !ok {
		return subx.CommandJob{}, ErrUnavailable
	}
	return provider.Retry(ctx, userID, id, confirmation)
}

func extractSubscriptions(raw json.RawMessage) ([]subscription.CreateInput, int, int) {
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return nil, 0, 0
	}
	result := make([]subscription.CreateInput, 0)
	seen := map[string]struct{}{}
	detected := 0
	rejected := 0
	walk(value, func(object map[string]any) {
		tmdbID := text(object, "tmdb_id", "tmdbId", "tmdbid")
		if tmdbID == "" {
			return
		}
		detected++
		title := text(object, "title", "name", "media_title")
		mediaType := normalizedMediaType(text(object, "media_type", "mediaType", "type"))
		year := integer(object, "year", "release_year")
		seasons := seasonValues(object)
		if title == "" || mediaType == "" || !positiveDigits(tmdbID) || year < 0 || year > 2100 || (mediaType == "series" && len(seasons) == 0) {
			rejected++
			return
		}
		if mediaType == "movie" {
			seasons = []int{0}
		}
		for _, season := range seasons {
			if season < 0 || season > 100 || (mediaType == "series" && season == 0) {
				rejected++
				continue
			}
			key := tmdbID + "\x00" + mediaType + "\x00" + strconv.Itoa(season)
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			seen[key] = struct{}{}
			sources := nativeSourceIDs(object)
			result = append(result, subscription.CreateInput{
				TMDBID: tmdbID, Title: title, OriginalTitle: text(object, "original_title", "originalTitle"),
				Year: year, MediaType: mediaType, Season: season, Policy: "once",
				Enabled: booleanDefault(object, true, "enabled", "is_enabled", "subscribed"), IntervalMinutes: 60,
				SourceIDs: sources, Preferences: subscription.Preferences{AllowUnknownSize: true},
			})
		}
	})
	return result, detected, rejected
}

func nativeSourceIDs(object map[string]any) []string {
	aliases := map[string]string{
		"dian": "dian", "点": "dian", "framehdr": "framehdr", "frame": "framehdr", "帧影": "framehdr",
		"gimy": "gimy", "guanying": "guanying", "观影": "guanying", "hdhive": "hdhive",
		"juying": "juying", "gather": "juying", "聚影": "juying", "mikan": "mikan", "sidhub": "sidhub",
	}
	seen := map[string]struct{}{}
	add := func(value string) {
		normalized := normalize(value)
		for alias, id := range aliases {
			if normalized == normalize(alias) {
				seen[id] = struct{}{}
				return
			}
		}
	}
	for _, name := range []string{"source", "source_id", "source_name", "site", "provider"} {
		if value := text(object, name); value != "" {
			add(value)
		}
	}
	if values, ok := field(object, "sources", "source_ids", "sites").([]any); ok {
		for _, value := range values {
			if text, ok := value.(string); ok {
				add(text)
			}
		}
	}
	if len(seen) == 0 {
		for _, id := range []string{"dian", "framehdr", "gimy", "guanying", "hdhive", "juying", "mikan", "sidhub"} {
			seen[id] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

func seasonValues(object map[string]any) []int {
	season := integer(object, "season", "season_number")
	if season > 0 {
		return []int{season}
	}
	value := field(object, "seasons", "season_numbers")
	array, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]int, 0, len(array))
	for _, item := range array {
		switch typed := item.(type) {
		case json.Number:
			value, _ := strconv.Atoi(typed.String())
			result = append(result, value)
		case float64:
			result = append(result, int(typed))
		case string:
			value, _ := strconv.Atoi(strings.TrimSpace(typed))
			result = append(result, value)
		}
	}
	return result
}

func walk(value any, visit func(map[string]any)) {
	switch typed := value.(type) {
	case map[string]any:
		visit(typed)
		for _, child := range typed {
			walk(child, visit)
		}
	case []any:
		for _, child := range typed {
			walk(child, visit)
		}
	}
}

func field(object map[string]any, names ...string) any {
	for key, value := range object {
		key = normalize(key)
		for _, name := range names {
			if key == normalize(name) {
				return value
			}
		}
	}
	return nil
}

func text(object map[string]any, names ...string) string {
	switch value := field(object, names...).(type) {
	case string:
		return strings.TrimSpace(value)
	case json.Number:
		return value.String()
	case float64:
		return strconv.FormatInt(int64(value), 10)
	default:
		return ""
	}
}

func integer(object map[string]any, names ...string) int {
	value, _ := strconv.Atoi(text(object, names...))
	return value
}

func booleanDefault(object map[string]any, fallback bool, names ...string) bool {
	value := field(object, names...)
	if value == nil {
		return fallback
	}
	if boolean, ok := value.(bool); ok {
		return boolean
	}
	parsed, err := strconv.ParseBool(text(object, names...))
	if err != nil {
		return fallback
	}
	return parsed
}

func normalizedMediaType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "movie", "film", "电影":
		return "movie"
	case "series", "tv", "tvshow", "电视剧", "剧集":
		return "series"
	default:
		return ""
	}
}

func positiveDigits(value string) bool {
	if value == "" || value[0] == '0' {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func normalize(value string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(value))
}
