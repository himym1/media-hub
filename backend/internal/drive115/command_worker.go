package drive115

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"media-hub/backend/internal/securepayload"
	"media-hub/backend/internal/store"
)

type CommandWorker struct {
	store    *store.Store
	codec    *securepayload.Codec
	executor *AuthService
	logger   *slog.Logger
	interval time.Duration
	now      func() time.Time
}

func NewCommandWorker(storage *store.Store, codec *securepayload.Codec, executor *AuthService, logger *slog.Logger) *CommandWorker {
	return &CommandWorker{store: storage, codec: codec, executor: executor, logger: logger, interval: 2 * time.Second, now: time.Now}
}

func (w *CommandWorker) Run(ctx context.Context) error {
	if w == nil || w.store == nil || w.codec == nil || w.executor == nil {
		return nil
	}
	if err := w.store.MarkInterruptedDrive115Commands(ctx, w.now()); err != nil {
		return err
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		if err := w.processOne(ctx); err != nil && w.logger != nil {
			w.logger.Error("115 command worker iteration failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (w *CommandWorker) processOne(ctx context.Context) error {
	job, ok, err := w.store.NextDrive115Command(ctx)
	if err != nil || !ok {
		return err
	}
	job, ok, err = w.store.BeginDrive115Command(ctx, job.ID, w.now())
	if err != nil || !ok {
		return err
	}
	var params map[string]any
	if err := w.codec.Open(job.PayloadToken, &params); err != nil {
		return w.finish(job, "failed", "invalid_payload", "无法读取加密命令参数", "加密命令参数无效")
	}
	err = w.executor.ExecuteFileCommand(ctx, job.Operation, params)
	if err == nil {
		result, sealErr := w.codec.Seal(map[string]bool{"ok": true})
		if sealErr != nil {
			return w.finish(job, "needs_attention", "result_encryption_failed", "115 已接受命令，但本地结果保存失败", "结果需要人工核对")
		}
		job.ResultToken = result
		return w.finish(job, "completed", "", "", "115 命令已完成")
	}
	var writeErr *WriteError
	if errors.As(err, &writeErr) && writeErr.Uncertain {
		return w.finish(job, "needs_attention", writeErr.Code, "提交结果未知，未自动重放", "提交结果未知，未自动重放")
	}
	code := "provider_rejected"
	if errors.Is(err, ErrUnauthorized) {
		code = "unauthorized"
	}
	if errors.As(err, &writeErr) && writeErr.Code != "" {
		code = writeErr.Code
	}
	return w.finish(job, "failed", code, "115 未接受命令", "115 命令失败")
}
func (w *CommandWorker) finish(job store.Drive115CommandJob, state, code, message, event string) error {
	job.State = state
	job.ErrorCode = code
	job.ErrorMessage = message
	job.UpdatedAt = w.now().UTC().Unix()
	return w.store.FinishDrive115Command(context.Background(), job, event)
}
