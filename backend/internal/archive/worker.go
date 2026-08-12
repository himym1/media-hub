package archive

import (
	"context"
	"errors"
	"log/slog"
	"media-hub/backend/internal/drive115"
	"media-hub/backend/internal/store"
	"time"
)

type Worker struct {
	service  *Service
	logger   *slog.Logger
	interval time.Duration
	now      func() time.Time
}

func NewWorker(service *Service, logger *slog.Logger) *Worker {
	return &Worker{service: service, logger: logger, interval: 2 * time.Second, now: time.Now}
}
func (w *Worker) Run(ctx context.Context) error {
	if w == nil || w.service == nil || w.service.codec == nil || w.service.provider == nil {
		return nil
	}
	if err := w.service.store.MarkInterruptedArchivePlans(ctx, w.now()); err != nil {
		return err
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		if err := w.processOne(ctx); err != nil && w.logger != nil {
			w.logger.Error("archive worker iteration failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
func (w *Worker) processOne(ctx context.Context) error {
	item, ok, err := w.service.store.NextArchivePlan(ctx)
	if err != nil || !ok {
		return err
	}
	item, ok, err = w.service.store.BeginArchivePlan(ctx, item.ID, w.now())
	if err != nil || !ok {
		return err
	}
	steps, err := w.service.steps(item)
	if err != nil {
		return w.finish(item, "failed", "invalid_payload", "无法读取加密归档计划", "归档计划无效")
	}
	for index := item.StepIndex; index < len(steps); index++ {
		step := steps[index]
		params := map[string]any{}
		if step.Operation == "rename" {
			params["fileId"] = step.FileID
			params["name"] = step.Name
		} else {
			params["fileIds"] = []string{step.FileID}
			params["targetParentId"] = step.TargetParentID
		}
		err := w.service.provider.ExecuteFileCommand(ctx, step.Operation, params)
		if err != nil {
			var writeErr *drive115.WriteError
			if errors.As(err, &writeErr) && writeErr.Uncertain {
				return w.finish(item, "needs_attention", "uncertain_result", "当前归档步骤结果未知，需要人工核对", "归档步骤结果未知，未自动继续")
			}
			return w.finish(item, "failed", "provider_rejected", "115 未接受当前归档步骤", "归档步骤失败")
		}
		item.StepIndex = index + 1
		if err := w.service.store.AdvanceArchivePlan(context.Background(), item.ID, item.StepIndex, w.now()); err != nil {
			return w.finish(item, "needs_attention", "progress_persist_failed", "115 已接受步骤，但本地进度保存失败", "归档进度需要人工核对")
		}
	}
	return w.finish(item, "completed", "", "", "归档计划已完成")
}
func (w *Worker) finish(item store.ArchivePlan, state, code, message, event string) error {
	item.State = state
	item.ErrorCode = code
	item.ErrorMessage = message
	item.UpdatedAt = w.now().UTC().Unix()
	return w.service.store.FinishArchivePlan(context.Background(), item, event)
}
