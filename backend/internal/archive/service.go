package archive

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"media-hub/backend/internal/drive115"
	"media-hub/backend/internal/securepayload"
	"media-hub/backend/internal/store"
	"regexp"
	"strings"
	"time"
)

var (
	ErrUnavailable = errors.New("archive unavailable")
	ErrInvalidPlan = errors.New("invalid archive plan")
)

type Provider interface {
	ListFiles(context.Context, string, int, int) ([]drive115.FileItem, int, error)
	ExecuteFileCommand(context.Context, string, map[string]any) error
}
type Service struct {
	store    *store.Store
	codec    *securepayload.Codec
	provider Provider
	now      func() time.Time
}
type Suggestion struct {
	FileID        string `json:"fileId"`
	CurrentName   string `json:"currentName"`
	SuggestedName string `json:"suggestedName"`
	Kind          string `json:"kind"`
	Confidence    string `json:"confidence"`
}
type Step struct {
	Operation      string `json:"operation"`
	FileID         string `json:"fileId"`
	Name           string `json:"name,omitempty"`
	TargetParentID string `json:"targetParentId,omitempty"`
}
type Plan struct {
	ID           string                   `json:"id"`
	State        string                   `json:"state"`
	StepIndex    int                      `json:"stepIndex"`
	StepTotal    int                      `json:"stepTotal"`
	Steps        []Step                   `json:"steps,omitempty"`
	ErrorCode    string                   `json:"errorCode,omitempty"`
	ErrorMessage string                   `json:"errorMessage,omitempty"`
	CreatedAt    time.Time                `json:"createdAt"`
	UpdatedAt    time.Time                `json:"updatedAt"`
	Events       []store.ArchivePlanEvent `json:"events,omitempty"`
}

func NewService(storage *store.Store, codec *securepayload.Codec, provider Provider) *Service {
	return &Service{store: storage, codec: codec, provider: provider, now: time.Now}
}
func (s *Service) Preview(ctx context.Context, parentID string) ([]Suggestion, error) {
	if s == nil || s.provider == nil {
		return nil, ErrUnavailable
	}
	if !numeric(parentID) {
		return nil, ErrInvalidPlan
	}
	items, _, err := s.provider.ListFiles(ctx, parentID, 200, 0)
	if err != nil {
		return nil, err
	}
	values := make([]Suggestion, 0, len(items))
	for _, item := range items {
		suggested, confidence := suggestName(item.Name)
		values = append(values, Suggestion{FileID: item.ID, CurrentName: item.Name, SuggestedName: suggested, Kind: item.Kind, Confidence: confidence})
	}
	return values, nil
}
func (s *Service) Create(ctx context.Context, userID int64, steps []Step) (Plan, error) {
	if s == nil || s.codec == nil {
		return Plan{}, ErrUnavailable
	}
	if len(steps) < 1 || len(steps) > 1000 {
		return Plan{}, ErrInvalidPlan
	}
	for _, step := range steps {
		if !validStep(step) {
			return Plan{}, ErrInvalidPlan
		}
	}
	token, err := s.codec.Seal(steps)
	if err != nil {
		return Plan{}, err
	}
	id, err := planID()
	if err != nil {
		return Plan{}, err
	}
	now := s.now().UTC()
	stored := store.ArchivePlan{ID: id, UserID: userID, PayloadToken: token, State: "awaiting_confirmation", StepTotal: len(steps), CreatedAt: now.Unix(), UpdatedAt: now.Unix()}
	if err := s.store.CreateArchivePlan(ctx, stored); err != nil {
		return Plan{}, err
	}
	return s.public(stored, true), nil
}
func (s *Service) List(ctx context.Context, userID int64, limit int) ([]Plan, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	items, err := s.store.ListArchivePlans(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	values := make([]Plan, 0, len(items))
	for _, item := range items {
		values = append(values, s.public(item, false))
	}
	return values, nil
}
func (s *Service) Get(ctx context.Context, userID int64, id string) (Plan, error) {
	item, err := s.store.ArchivePlan(ctx, userID, id)
	if err != nil {
		return Plan{}, err
	}
	value := s.public(item, true)
	value.Events, err = s.store.ArchivePlanEvents(ctx, userID, id)
	return value, err
}
func (s *Service) Confirm(ctx context.Context, userID int64, id, confirmation string) (Plan, error) {
	item, err := s.store.ConfirmArchivePlan(ctx, userID, id, confirmation, s.now())
	if err != nil {
		return Plan{}, err
	}
	return s.public(item, true), nil
}
func (s *Service) Retry(ctx context.Context, userID int64, id, confirmation string) (Plan, error) {
	item, err := s.store.RetryArchivePlan(ctx, userID, id, confirmation, s.now())
	if err != nil {
		return Plan{}, err
	}
	return s.public(item, true), nil
}
func (s *Service) steps(item store.ArchivePlan) ([]Step, error) {
	var values []Step
	if err := s.codec.Open(item.PayloadToken, &values); err != nil {
		return nil, err
	}
	return values, nil
}
func (s *Service) public(item store.ArchivePlan, include bool) Plan {
	value := Plan{ID: item.ID, State: item.State, StepIndex: item.StepIndex, StepTotal: item.StepTotal, ErrorCode: item.ErrorCode, ErrorMessage: item.ErrorMessage, CreatedAt: time.Unix(item.CreatedAt, 0).UTC(), UpdatedAt: time.Unix(item.UpdatedAt, 0).UTC()}
	if include {
		value.Steps, _ = s.steps(item)
	}
	return value
}
func validStep(step Step) bool {
	if !numeric(step.FileID) {
		return false
	}
	switch step.Operation {
	case "rename":
		return strings.TrimSpace(step.Name) != "" && len(step.Name) <= 255 && !strings.ContainsAny(step.Name, "/\\\x00")
	case "move":
		return numeric(step.TargetParentID)
	default:
		return false
	}
}
func numeric(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

var yearPattern = regexp.MustCompile(`(?i)\b((?:19|20)\d{2})\b`)
var releasePattern = regexp.MustCompile(`(?i)\b(?:2160p|1080p|720p|bluray|web[- .]?dl|webrip|hdtv|x26[45]|hevc|av1|hdr10\+?|dolby[ .]?vision|dovi|remux)\b.*$`)

func suggestName(name string) (string, string) {
	extension := ""
	base := name
	if index := strings.LastIndex(name, "."); index > 0 {
		candidate := strings.ToLower(name[index:])
		if candidate == ".mkv" || candidate == ".mp4" || candidate == ".avi" || candidate == ".mov" || candidate == ".m4v" || candidate == ".ts" || candidate == ".iso" || candidate == ".strm" {
			extension, base = name[index:], name[:index]
		}
	}
	clean := strings.NewReplacer(".", " ", "_", " ").Replace(base)
	clean = releasePattern.ReplaceAllString(clean, "")
	clean = strings.Join(strings.Fields(clean), " ")
	year := yearPattern.FindString(clean)
	if year != "" {
		clean = strings.TrimSpace(strings.Replace(clean, year, "", 1))
		if clean != "" {
			return clean + " (" + year + ")" + extension, "review"
		}
	}
	if clean == "" {
		return name, "review"
	}
	return clean + extension, "review"
}
func planID() (string, error) {
	value := make([]byte, 18)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "arc_" + base64.RawURLEncoding.EncodeToString(value), nil
}
