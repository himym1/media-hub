package drive115

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"media-hub/backend/internal/securepayload"
	"media-hub/backend/internal/store"
)

var (
	ErrInvalidCommand     = errors.New("invalid 115 command")
	ErrCommandUnavailable = errors.New("115 command service unavailable")
)

type CommandService struct {
	store *store.Store
	codec *securepayload.Codec
	now   func() time.Time
}

type CommandInput struct {
	Operation string         `json:"operation"`
	Params    map[string]any `json:"params"`
}

type Command struct {
	ID           string                       `json:"id"`
	Operation    string                       `json:"operation"`
	State        string                       `json:"state"`
	Attempts     int                          `json:"attempts"`
	ErrorCode    string                       `json:"errorCode,omitempty"`
	ErrorMessage string                       `json:"errorMessage,omitempty"`
	CreatedAt    time.Time                    `json:"createdAt"`
	UpdatedAt    time.Time                    `json:"updatedAt"`
	Events       []store.Drive115CommandEvent `json:"events,omitempty"`
}

func NewCommandService(storage *store.Store, codec *securepayload.Codec) *CommandService {
	return &CommandService{store: storage, codec: codec, now: time.Now}
}

func (s *CommandService) Create(ctx context.Context, userID int64, idempotencyKey string, input CommandInput) (Command, error) {
	if s == nil || s.store == nil || s.codec == nil {
		return Command{}, ErrCommandUnavailable
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" || len(idempotencyKey) > 200 || validateCommandInput(input) != nil {
		return Command{}, ErrInvalidCommand
	}
	payload, err := json.Marshal(input.Params)
	if err != nil {
		return Command{}, ErrInvalidCommand
	}
	token, err := s.codec.Seal(input.Params)
	if err != nil {
		return Command{}, err
	}
	id, err := commandID()
	if err != nil {
		return Command{}, err
	}
	now := s.now().UTC()
	state := "queued"
	if input.Operation == "delete" {
		state = "awaiting_confirmation"
	}
	digest := sha256.Sum256(append([]byte(input.Operation+"\x00"), payload...))
	job, _, err := s.store.CreateDrive115Command(ctx, store.Drive115CommandJob{ID: id, UserID: userID, Operation: input.Operation, IdempotencyKey: idempotencyKey, RequestHash: digest[:], PayloadToken: token, State: state, CreatedAt: now.Unix(), UpdatedAt: now.Unix()})
	if err != nil {
		return Command{}, err
	}
	return commandFromJob(job), nil
}

func (s *CommandService) List(ctx context.Context, userID int64, limit int) ([]Command, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	jobs, err := s.store.ListDrive115Commands(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	values := make([]Command, 0, len(jobs))
	for _, job := range jobs {
		values = append(values, commandFromJob(job))
	}
	return values, nil
}
func (s *CommandService) Get(ctx context.Context, userID int64, id string) (Command, error) {
	job, err := s.store.Drive115Command(ctx, userID, id)
	if err != nil {
		return Command{}, err
	}
	value := commandFromJob(job)
	value.Events, err = s.store.Drive115CommandEvents(ctx, userID, id)
	return value, err
}
func (s *CommandService) Confirm(ctx context.Context, userID int64, id, confirmation string) (Command, error) {
	job, err := s.store.ConfirmDrive115Command(ctx, userID, id, confirmation, s.now())
	if err != nil {
		return Command{}, err
	}
	return commandFromJob(job), nil
}
func (s *CommandService) Retry(ctx context.Context, userID int64, id, confirmation string) (Command, error) {
	job, err := s.store.RetryDrive115Command(ctx, userID, id, confirmation, s.now())
	if err != nil {
		return Command{}, err
	}
	return commandFromJob(job), nil
}

func validateCommandInput(input CommandInput) error {
	id := func(key string) bool {
		value, ok := input.Params[key].(string)
		if !ok {
			return false
		}
		value = strings.TrimSpace(value)
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
	name := func() bool {
		value, ok := input.Params["name"].(string)
		value = strings.TrimSpace(value)
		return ok && value != "" && len(value) <= 255 && !strings.ContainsAny(value, "/\\\x00")
	}
	ids := func() bool {
		values := stringSlice(input.Params["fileIds"])
		if len(values) < 1 || len(values) > 100 {
			return false
		}
		for _, value := range values {
			for _, r := range value {
				if r < '0' || r > '9' {
					return false
				}
			}
		}
		return true
	}
	switch input.Operation {
	case "create_folder":
		if id("parentId") && name() {
			return nil
		}
	case "move":
		if ids() && id("targetParentId") {
			return nil
		}
	case "rename":
		if id("fileId") && name() {
			return nil
		}
	case "delete":
		if ids() {
			return nil
		}
	}
	return ErrInvalidCommand
}

func commandFromJob(job store.Drive115CommandJob) Command {
	return Command{ID: job.ID, Operation: job.Operation, State: job.State, Attempts: job.Attempts, ErrorCode: job.ErrorCode, ErrorMessage: job.ErrorMessage, CreatedAt: time.Unix(job.CreatedAt, 0).UTC(), UpdatedAt: time.Unix(job.UpdatedAt, 0).UTC()}
}
func commandID() (string, error) {
	value := make([]byte, 18)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "115_" + base64.RawURLEncoding.EncodeToString(value), nil
}
