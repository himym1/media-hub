package localupload

import (
	"context"
	"errors"
	"log/slog"
	"media-hub/backend/internal/drive115"
	"media-hub/backend/internal/store"
	"sync"
	"time"
)

type Worker struct {
	service  *Service
	uploader Uploader
	logger   *slog.Logger
	interval time.Duration
	now      func() time.Time
}

func NewWorker(service *Service, uploader Uploader, logger *slog.Logger) *Worker {
	return &Worker{service: service, uploader: uploader, logger: logger, interval: 2 * time.Second, now: time.Now}
}
func (w *Worker) Run(ctx context.Context) error {
	if w == nil || w.service == nil || w.service.codec == nil || w.uploader == nil || len(w.service.roots) == 0 {
		return nil
	}
	if err := w.service.store.MarkInterruptedLocalUploads(ctx, w.now()); err != nil {
		return err
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		if err := w.processOne(ctx); err != nil && w.logger != nil {
			w.logger.Error("local upload worker iteration failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
func (w *Worker) processOne(ctx context.Context) error {
	item, ok, err := w.service.store.NextLocalUpload(ctx)
	if err != nil || !ok {
		return err
	}
	item, ok, err = w.service.store.TransitionLocalUpload(ctx, item.ID, "queued", "hashing", "正在校验本地文件", w.now())
	if err != nil || !ok {
		return err
	}
	payload, path, err := w.service.payload(item)
	if err != nil {
		if errors.Is(err, ErrSourceChanged) {
			return w.finish(item, "failed", "source_changed", "本地文件在任务创建后发生变化，请新建上传任务", "本地文件已变化")
		}
		return w.finish(item, "failed", "invalid_path", "本地文件不在允许目录中", "本地文件校验失败")
	}
	item, ok, err = w.service.store.TransitionLocalUpload(ctx, item.ID, "hashing", "submitting_init", "正在初始化 115 上传", w.now())
	if err != nil || !ok {
		return err
	}
	var once sync.Once
	progress := func(done, total int64) {
		once.Do(func() {
			if next, changed, _ := w.service.store.TransitionLocalUpload(context.Background(), item.ID, "submitting_init", "uploading", "正在上传到 115", w.now()); changed {
				item = next
			}
		})
		_ = w.service.store.UpdateLocalUploadProgress(context.Background(), item.ID, done, total, w.now())
	}
	err = w.uploader.UploadLocalFile(ctx, item.UserID, path, payload.DestinationID, progress)
	if err == nil {
		item.BytesDone = item.BytesTotal
		return w.finish(item, "completed", "", "", "上传已完成")
	}
	if errors.Is(err, drive115.ErrUploadUncertain) || item.State == "uploading" {
		return w.finish(item, "needs_attention", "uncertain_result", "上传结果未知，需要人工核对后再重试", "上传结果未知，未自动重放")
	}
	return w.finish(item, "failed", "upload_failed", "上传未开始或被 provider 拒绝", "上传失败")
}
func (w *Worker) finish(item store.LocalUploadJob, state, code, message, event string) error {
	item.State = state
	item.ErrorCode = code
	item.ErrorMessage = message
	item.UpdatedAt = w.now().UTC().Unix()
	return w.service.store.FinishLocalUpload(context.Background(), item, event)
}
