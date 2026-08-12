package subx

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"media-hub/backend/internal/securepayload"
	"media-hub/backend/internal/store"
)

var ErrInvalidIdempotency = errors.New("invalid SubX command idempotency key")

var idempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,100}$`)

type CommandJob struct {
	ID           string          `json:"id"`
	OperationID  string          `json:"operationId"`
	State        string          `json:"state"`
	Attempts     int             `json:"attempts"`
	ErrorCode    string          `json:"errorCode,omitempty"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	Retryable    bool            `json:"retryable"`
	Result       json.RawMessage `json:"result,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}

type CommandEvent struct {
	ID        int64     `json:"id"`
	State     string    `json:"state"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type CommandDetail struct {
	CommandJob
	Events []CommandEvent `json:"events"`
}

type Service struct {
	store  *store.Store
	client *Client
	codec  *securepayload.Codec
	now    func() time.Time
	wake   chan struct{}
	done   chan struct{}
}

func NewService(dataStore *store.Store, client *Client, codec *securepayload.Codec) *Service {
	return &Service{
		store: dataStore, client: client, codec: codec, now: time.Now,
		wake: make(chan struct{}, 1), done: make(chan struct{}),
	}
}

func (s *Service) Configured() bool {
	return s != nil && s.client != nil && s.client.Configured()
}

func (s *Service) PendingCommands(ctx context.Context, userID int64) (int, error) {
	if s == nil || s.store == nil {
		return 0, ErrNotConfigured
	}
	return s.store.CountBlockingSubXCommands(ctx, userID)
}

func (s *Service) ExportContents(ctx context.Context) (json.RawMessage, error) {
	if s == nil || s.client == nil {
		return nil, ErrNotConfigured
	}
	return s.client.readInternal(ctx, "contents.backup.export", Invocation{})
}

func (s *Service) enqueueInternal(ctx context.Context, userID int64, operationID, idempotencyKey string, invocation Invocation) (CommandJob, bool, error) {
	if s == nil || s.client == nil || s.store == nil || s.codec == nil || !s.client.Configured() {
		return CommandJob{}, false, ErrNotConfigured
	}
	operation, ok := LookupOperation(operationID)
	if !ok || !operation.Command {
		return CommandJob{}, false, ErrUnknownOperation
	}
	if err := validateInvocation(operation, invocation); err != nil {
		return CommandJob{}, false, err
	}
	if !idempotencyPattern.MatchString(idempotencyKey) {
		return CommandJob{}, false, ErrInvalidIdempotency
	}
	payloadToken, err := s.codec.Seal(invocation)
	if err != nil {
		return CommandJob{}, false, ErrNotConfigured
	}
	digest, err := securepayload.Digest(struct {
		OperationID string     `json:"operationId"`
		Invocation  Invocation `json:"invocation"`
	}{operationID, invocation})
	if err != nil {
		return CommandJob{}, false, err
	}
	id, err := commandID()
	if err != nil {
		return CommandJob{}, false, err
	}
	now := s.now().UTC().Unix()
	stored, created, err := s.store.CreateSubXCommand(ctx, store.SubXCommandJob{
		ID: id, UserID: userID, OperationID: operationID, IdempotencyKey: idempotencyKey,
		RequestHash: digest, PayloadToken: payloadToken, State: "queued", CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return CommandJob{}, false, err
	}
	if created {
		s.notify()
	}
	job, err := s.publicJob(stored, false)
	return job, created, err
}

func (s *Service) Get(ctx context.Context, userID int64, id string) (CommandDetail, error) {
	if s == nil || s.store == nil {
		return CommandDetail{}, ErrNotConfigured
	}
	stored, err := s.store.SubXCommand(ctx, userID, id)
	if err != nil {
		return CommandDetail{}, err
	}
	job, err := s.publicJob(stored, true)
	if err != nil {
		return CommandDetail{}, err
	}
	storedEvents, err := s.store.SubXCommandEvents(ctx, userID, id)
	if err != nil {
		return CommandDetail{}, err
	}
	events := make([]CommandEvent, 0, len(storedEvents))
	for _, event := range storedEvents {
		events = append(events, CommandEvent{
			ID: event.ID, State: event.State, Message: event.Message,
			CreatedAt: time.Unix(event.CreatedAt, 0).UTC(),
		})
	}
	return CommandDetail{CommandJob: job, Events: events}, nil
}

func (s *Service) List(ctx context.Context, userID int64, limit int) ([]CommandJob, error) {
	if s == nil || s.store == nil {
		return nil, ErrNotConfigured
	}
	stored, err := s.store.ListSubXCommands(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	jobs := make([]CommandJob, 0, len(stored))
	for _, item := range stored {
		job, err := s.publicJob(item, false)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (s *Service) Retry(ctx context.Context, userID int64, id, confirmation string) (CommandJob, error) {
	if confirmation != id {
		return CommandJob{}, ErrInvalidInvocation
	}
	stored, err := s.store.RetrySubXCommand(ctx, userID, id, s.now())
	if err != nil {
		return CommandJob{}, err
	}
	s.notify()
	return s.publicJob(stored, false)
}

func (s *Service) Start(ctx context.Context) error {
	if s == nil || s.store == nil {
		return ErrNotConfigured
	}
	if err := s.store.MarkInterruptedSubXCommands(ctx, s.now()); err != nil {
		return err
	}
	go func() {
		defer close(s.done)
		s.run(ctx)
	}()
	return nil
}

func (s *Service) Wait(ctx context.Context) error {
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		for {
			job, found, err := s.store.NextQueuedSubXCommand(ctx)
			if err != nil || !found {
				break
			}
			if err := s.process(ctx, job); err != nil {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-ticker.C:
		}
	}
}

func (s *Service) process(ctx context.Context, job store.SubXCommandJob) error {
	begun, ok, err := s.store.BeginSubXCommand(ctx, job.ID, s.now())
	if err != nil || !ok {
		return err
	}
	var invocation Invocation
	if err := s.codec.Open(begun.PayloadToken, &invocation); err != nil {
		begun.State = "failed"
		begun.ErrorCode = "invalid_encrypted_payload"
		begun.ErrorMessage = "命令参数无法解密"
		begun.UpdatedAt = s.now().UTC().Unix()
		return s.store.FinishSubXCommand(ctx, begun, "submitting", "命令参数无效")
	}
	result, executeErr := s.client.ExecuteCommand(ctx, begun.OperationID, invocation)
	begun.UpdatedAt = s.now().UTC().Unix()
	if executeErr == nil {
		resultToken, sealErr := s.codec.Seal(result)
		if sealErr != nil {
			begun.State = "needs_attention"
			begun.ErrorCode = "result_encryption_failed"
			begun.ErrorMessage = "命令已提交，但结果无法安全保存"
			return s.store.FinishSubXCommand(ctx, begun, "submitting", "结果需要人工确认")
		}
		begun.State = "completed"
		begun.ResultToken = resultToken
		return s.store.FinishSubXCommand(ctx, begun, "submitting", "命令执行完成")
	}
	switch {
	case errors.Is(executeErr, ErrUnauthorized), errors.Is(executeErr, ErrInvalidInvocation), errors.Is(executeErr, ErrUnknownOperation), errors.Is(executeErr, ErrNotConfigured):
		begun.State = "failed"
		begun.ErrorCode = commandErrorCode(executeErr)
		begun.ErrorMessage = "命令被拒绝，未确认产生外部修改"
		begun.Retryable = errors.Is(executeErr, ErrUnauthorized)
		return s.store.FinishSubXCommand(ctx, begun, "submitting", "命令执行失败")
	default:
		begun.State = "needs_attention"
		begun.ErrorCode = commandErrorCode(executeErr)
		begun.ErrorMessage = "命令提交结果不确定，未自动重放"
		return s.store.FinishSubXCommand(ctx, begun, "submitting", "提交结果需要人工确认")
	}
}

func (s *Service) publicJob(item store.SubXCommandJob, includeResult bool) (CommandJob, error) {
	job := CommandJob{
		ID: item.ID, OperationID: item.OperationID, State: item.State, Attempts: item.Attempts,
		ErrorCode: item.ErrorCode, ErrorMessage: item.ErrorMessage, Retryable: item.Retryable,
		CreatedAt: time.Unix(item.CreatedAt, 0).UTC(), UpdatedAt: time.Unix(item.UpdatedAt, 0).UTC(),
	}
	if includeResult && item.ResultToken != "" {
		var raw json.RawMessage
		if err := s.codec.Open(item.ResultToken, &raw); err != nil {
			return CommandJob{}, err
		}
		job.Result = sanitizeRaw(raw)
	}
	return job, nil
}

func (s *Service) commandResult(ctx context.Context, userID int64, id string) (store.SubXCommandJob, json.RawMessage, error) {
	item, err := s.store.SubXCommand(ctx, userID, id)
	if err != nil || item.ResultToken == "" {
		return item, nil, err
	}
	var result json.RawMessage
	if err := s.codec.Open(item.ResultToken, &result); err != nil {
		return item, nil, err
	}
	return item, result, nil
}

func sanitizeRaw(raw json.RawMessage) json.RawMessage {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	result, _ := json.Marshal(sanitize(value, 0))
	return result
}

func commandErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrUnauthorized):
		return "subx_unauthorized"
	case errors.Is(err, ErrInvalidInvocation):
		return "invalid_invocation"
	case errors.Is(err, ErrUnknownOperation):
		return "unknown_operation"
	case errors.Is(err, ErrNotConfigured):
		return "subx_unconfigured"
	case errors.Is(err, ErrInvalidResponse):
		return "subx_invalid_response"
	default:
		return "subx_unavailable"
	}
}

func commandID() (string, error) {
	value := make([]byte, 18)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (s *Service) notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}
