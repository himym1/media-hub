package workflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"media-hub/backend/internal/config"
	"media-hub/backend/internal/emby"
	"media-hub/backend/internal/search"
	"media-hub/backend/internal/selection"
	"media-hub/backend/internal/store"
	"media-hub/backend/internal/strm"
	"media-hub/backend/internal/wecom"
)

const (
	workerPollInterval = 5 * time.Second
	transferPollDelay  = 5 * time.Second
	offlinePollDelay   = 15 * time.Second
	offlineWaitTimeout = 10 * time.Minute
	syncPollDelay      = 15 * time.Second
	maxAutomaticTries  = 5
)

func (s *Service) Start(ctx context.Context) error {
	if err := s.store.MarkInterruptedSyncSubmissions(ctx, s.now()); err != nil {
		return err
	}
	if s.notifier != nil && s.notifier.Configured() {
		if err := s.store.MarkInterruptedNotifications(ctx, s.now()); err != nil {
			return err
		}
		if err := s.store.EnsureTerminalNotifications(ctx, s.now()); err != nil {
			return err
		}
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
	ticker := time.NewTicker(workerPollInterval)
	defer ticker.Stop()
	for {
		if s.codec != nil {
			for {
				job, found, err := s.store.NextRunnableTransfer(ctx, s.now())
				if err != nil || !found {
					break
				}
				if err := s.processJob(ctx, job); err != nil {
					break
				}
			}
		}
		if s.notifier != nil && s.notifier.Configured() {
			if err := s.store.EnsureTerminalNotifications(ctx, s.now()); err == nil {
				for {
					notification, found, readErr := s.store.NextTransferNotification(ctx, s.now())
					if readErr != nil || !found {
						break
					}
					if err := s.processNotification(ctx, notification); err != nil {
						break
					}
				}
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

func (s *Service) processNotification(ctx context.Context, item store.TransferNotification) error {
	begun, err := s.store.BeginTransferNotification(ctx, item, s.now())
	if err != nil || !begun {
		return err
	}
	message := s.notificationMessage(ctx, item)
	unknown, sendErr := s.notifier.Send(ctx, message)
	if sendErr == nil {
		return s.store.FinishTransferNotification(ctx, item, "sent", "企业微信通知已发送", s.now())
	}
	if unknown {
		return s.store.FinishTransferNotification(ctx, item, "needs_attention", "企业微信通知结果未知，未自动重发", s.now())
	}
	if item.Attempts+1 < maxAutomaticTries {
		return s.store.RetryTransferNotification(
			ctx, item, s.now().UTC().Add(backoff(item.Attempts+1)), s.now(),
		)
	}
	return s.store.FinishTransferNotification(ctx, item, "needs_attention", wecomFailureMessage(sendErr), s.now())
}

func wecomFailureMessage(err error) string {
	if code := wecom.ErrorCode(err); code != 0 {
		return fmt.Sprintf("企业微信通知发送失败（%d）", code)
	}
	return "企业微信通知发送失败"
}

func (s *Service) notificationMessage(ctx context.Context, item store.TransferNotification) string {
	title := "《" + item.Title + "》"
	switch item.EventType {
	case "completed":
		if sourceID, err := s.store.TransferJobSourceID(ctx, item.JobID); err == nil && sourceID == "moviepilot" {
			return "Media Hub\n" + title + "已提交到 MoviePilot 下载"
		}
		return "Media Hub\n" + title + "已完成转存并可播放"
	case "needs_attention":
		if item.ErrorMessage != "" {
			return "Media Hub\n" + title + "需要人工确认\n" + item.ErrorMessage
		}
		return "Media Hub\n" + title + "需要人工确认，请查看任务详情"
	default:
		if item.ErrorMessage != "" {
			return "Media Hub\n" + title + "处理失败\n" + item.ErrorMessage
		}
		return "Media Hub\n" + title + "处理失败，请查看任务详情"
	}
}

func (s *Service) processJob(ctx context.Context, job store.TransferJob) error {
	switch job.State {
	case "queued":
		if _, ok := s.search.DownloadSource(job.SourceID); ok {
			job.State = "transferring"
			job.NextAttemptAt = 0
			return s.save(ctx, &job, "queued", "开始提交 MoviePilot 下载")
		}
		job.State = "transferring"
		job.NextAttemptAt = 0
		return s.save(ctx, &job, "queued", "开始资源转存")
	case "retry_wait":
		resume := job.ResumeState
		if resume == "" {
			resume = "queued"
		}
		job.State = resume
		job.ResumeState = ""
		job.NextAttemptAt = 0
		return s.save(ctx, &job, "retry_wait", "继续处理")
	case "transferring":
		if _, ok := s.search.DownloadSource(job.SourceID); ok {
			return s.processDownload(ctx, job)
		}
		return s.processTransfer(ctx, job)
	case "transferred":
		return s.submitSync(ctx, job)
	case "syncing":
		return s.pollSync(ctx, job)
	case "refreshing_emby":
		return s.refreshEmby(ctx, job)
	case "indexing_emby":
		return s.pollEmbyIndex(ctx, job)
	case "verifying_playback":
		return s.verifyPlayback(ctx, job)
	default:
		return nil
	}
}

func (s *Service) processTransfer(ctx context.Context, job store.TransferJob) error {
	payload, err := s.codec.Decode(job.SelectionToken)
	if err != nil {
		return s.fail(ctx, &job, "transferring", "selection_expired", "资源选择已过期", false, "")
	}
	target, ok := s.workflowConfiguration().Target(payload.MediaType)
	if !ok {
		return s.fail(ctx, &job, "transferring", "target_unconfigured", "转存目标未配置", true, "transferring")
	}
	source, ok := s.search.TransferSource(payload.SourceID)
	if !ok {
		return s.fail(ctx, &job, "transferring", "source_unsupported", "资源源不支持转存", false, "")
	}
	provider := selection.ProviderPayload{}
	if job.ProviderToken != "" {
		provider, err = s.codec.DecodeProvider(job.ProviderToken)
		if err != nil {
			return s.fail(ctx, &job, "transferring", "provider_state_invalid", "资源转存状态无法解密", false, "")
		}
	}
	inspect := s.folderVideos()
	canInspect := inspect != nil && provider.FileID != "" && !provider.IsFile
	startedTransfer := provider.OperationID == ""
	var result search.TransferResult
	if startedTransfer {
		job.Attempts++
		job.NextAttemptAt = 0
		if err := s.save(ctx, &job, "transferring", ""); err != nil {
			return err
		}
		result, err = source.StartTransfer(ctx, search.TransferRequest{
			UserID: job.UserID, Title: job.Title, MediaType: payload.MediaType, Reference: payload.Reference, DestinationID: target.DestinationID,
			IdempotencyKey: job.ID + "_transfer",
		})
	} else if canInspect {
		result = search.TransferResult{
			OperationID: provider.OperationID, FileID: provider.FileID, Path: provider.Path, IsFile: provider.IsFile, Status: "pending",
		}
	} else {
		job.Attempts++
		job.NextAttemptAt = 0
		if err := s.save(ctx, &job, "transferring", ""); err != nil {
			return err
		}
		result, err = source.TransferStatus(ctx, job.UserID, provider.OperationID)
	}
	if err != nil {
		return s.handleSourceFailure(ctx, &job, err)
	}
	providerToken, err := s.codec.EncodeProvider(selection.ProviderPayload{
		OperationID: result.OperationID, FileID: result.FileID, Path: result.Path, IsFile: result.IsFile,
	})
	if err != nil {
		return err
	}
	job.ProviderToken = providerToken
	if result.FileID != "" && !result.IsFile && inspect != nil {
		ready, waitErr := s.waitForOfflineVideos(ctx, &job, inspect, result.FileID, startedTransfer)
		if waitErr != nil || !ready {
			return waitErr
		}
	} else if result.Status == "pending" {
		job.NextAttemptAt = s.now().UTC().Add(transferPollDelay).Unix()
		job.ErrorCode = ""
		job.ErrorMessage = ""
		return s.save(ctx, &job, "transferring", "")
	}

	job.State = "transferred"
	job.Attempts = 0
	job.NextAttemptAt = 0
	job.ErrorCode = ""
	job.ErrorMessage = ""
	return s.save(ctx, &job, "transferring", "资源转存完成")
}

func (s *Service) waitForOfflineVideos(ctx context.Context, job *store.TransferJob, inspect FolderVideoInspector, folderID string, announce bool) (bool, error) {
	ready, err := inspect(ctx, folderID)
	if err != nil {
		if errors.Is(err, strm.ErrAuthExpired) {
			return false, s.fail(ctx, job, job.State, "strm_auth_expired", "115 授权已失效，请在概览页重新扫码后再重试任务", true, "transferring")
		}
		return false, s.handleSourceFailure(ctx, job, search.Failure{Code: "source_unavailable", Message: "无法确认 115 离线是否到账", Retryable: true})
	}
	if ready {
		return true, nil
	}
	if s.now().UTC().Sub(time.Unix(job.CreatedAt, 0).UTC()) >= offlineWaitTimeout {
		return false, s.fail(ctx, job, job.State, "offline_incomplete", "115 离线未完成，目录里还没有视频", true, "transferring")
	}
	expectedState := job.State
	if expectedState == "transferred" {
		job.State = "transferring"
	}
	job.Attempts = 0
	job.NextAttemptAt = s.now().UTC().Add(offlinePollDelay).Unix()
	job.ErrorCode = ""
	job.ErrorMessage = ""
	message := ""
	if announce {
		message = "已提交 115 离线，等待视频到账"
	}
	return false, s.save(ctx, job, expectedState, message)
}

func (s *Service) handleSourceFailure(ctx context.Context, job *store.TransferJob, err error) error {
	var failure search.Failure
	if !errors.As(err, &failure) {
		failure = search.Failure{Code: "source_unavailable", Message: "资源源连接失败", Retryable: true}
	}
	from := job.State
	if failure.Code == "source_submission_unknown" || failure.Code == "source_access_unknown" {
		job.State = "needs_attention"
		job.ResumeState = s.downloadOrTransferResume(*job)
		job.ErrorCode = failure.Code
		job.ErrorMessage = failure.Message
		job.Retryable = true
		job.NextAttemptAt = 0
		eventMessage := "转存结果需要人工确认"
		if failure.Code == "source_access_unknown" {
			eventMessage = "资源访问结果需要人工确认"
		}
		return s.save(ctx, job, from, eventMessage)
	}
	if failure.Retryable && job.Attempts < maxAutomaticTries {
		job.ErrorCode = failure.Code
		job.ErrorMessage = failure.Message
		job.NextAttemptAt = s.now().UTC().Add(backoff(job.Attempts)).Unix()
		return s.save(ctx, job, from, "等待自动重试")
	}
	return s.fail(ctx, job, from, failure.Code, failure.Message, failure.Retryable, s.downloadOrTransferResume(*job))
}

func (s *Service) downloadOrTransferResume(store.TransferJob) string {
	return "transferring"
}

func (s *Service) processDownload(ctx context.Context, job store.TransferJob) error {
	payload, err := s.codec.Decode(job.SelectionToken)
	if err != nil {
		return s.fail(ctx, &job, "transferring", "selection_expired", "资源选择已过期", false, "")
	}
	source, ok := s.search.DownloadSource(payload.SourceID)
	if !ok {
		return s.fail(ctx, &job, "transferring", "source_unsupported", "资源源不支持下载", false, "")
	}
	job.Attempts++
	job.NextAttemptAt = 0
	if err := s.save(ctx, &job, "transferring", ""); err != nil {
		return err
	}
	if err := source.StartDownload(ctx, search.DownloadRequest{
		UserID: job.UserID, Title: job.Title, MediaType: payload.MediaType, Reference: payload.Reference,
	}); err != nil {
		return s.handleSourceFailure(ctx, &job, err)
	}
	job.State = "completed"
	job.Attempts = 0
	job.NextAttemptAt = 0
	job.ErrorCode = ""
	job.ErrorMessage = ""
	job.Retryable = false
	return s.save(ctx, &job, "transferring", "已提交到 MoviePilot 下载")
}

func (s *Service) submitSync(ctx context.Context, job store.TransferJob) error {
	workflowConfiguration := s.workflowConfiguration()
	target, ok := workflowConfiguration.Target(job.MediaType)
	if !ok {
		return s.fail(ctx, &job, "transferred", "target_unconfigured", "同步目标未配置", true, "transferred")
	}
	provider, err := s.codec.DecodeProvider(job.ProviderToken)
	if err != nil || provider.FileID == "" {
		return s.fail(ctx, &job, "transferred", "provider_state_invalid", "资源转存结果无法解密", false, "")
	}
	sourcePath := provider.Path
	isFile := provider.IsFile
	if s.resolveSourcePath != nil {
		sourcePath, err = s.resolveSourcePath(ctx, provider.FileID)
		if err != nil || sourcePath == "" {
			return s.fail(ctx, &job, "transferred", "sync_source_path_unavailable", "无法读取 115 同步路径", true, "transferred")
		}
		isFile = false
	}
	if sourcePath == "" {
		return s.fail(ctx, &job, "transferred", "provider_state_invalid", "资源转存结果无法解密", false, "")
	}
	if s.validateTransfer != nil && !isFile {
		if err := s.validateTransfer(ctx, job.MediaType, provider.FileID); err != nil {
			return s.fail(ctx, &job, "transferred", "source_identity_mismatch", err.Error(), false, "transferred")
		}
	}
	if inspect := s.folderVideos(); inspect != nil && !isFile && provider.FileID != "" {
		ready, waitErr := s.waitForOfflineVideos(ctx, &job, inspect, provider.FileID, true)
		if waitErr != nil || !ready {
			return waitErr
		}
	}
	desiredName := libraryEntryName(job.Title, job.Year, isFile, sourcePath)
	if s.renameSource != nil && shouldRenameTransferredFolder(provider.FileID, target.DestinationID, sourcePath, desiredName) {
		if err := s.renameSource(ctx, provider.FileID, desiredName); err != nil {
			if saveErr := s.save(ctx, &job, "transferred", "标准化目录名失败，继续同步"); saveErr != nil {
				return saveErr
			}
		} else {
			if s.resolveSourcePath != nil {
				if refreshed, refreshErr := s.resolveSourcePath(ctx, provider.FileID); refreshErr == nil && refreshed != "" {
					sourcePath = refreshed
					isFile = false
				} else {
					sourcePath = path.Join(path.Dir(strings.TrimRight(sourcePath, "/")), desiredName)
				}
			} else {
				sourcePath = path.Join(path.Dir(strings.TrimRight(sourcePath, "/")), desiredName)
			}
			provider.Path = sourcePath
			provider.IsFile = isFile
			if token, encodeErr := s.codec.EncodeProvider(provider); encodeErr == nil {
				job.ProviderToken = token
			}
			if saveErr := s.save(ctx, &job, "transferred", "已标准化网盘目录名"); saveErr != nil {
				return saveErr
			}
		}
	}
	return s.submitBuiltinSync(ctx, job, provider, sourcePath, isFile, target)
}

func (s *Service) submitBuiltinSync(
	ctx context.Context,
	job store.TransferJob,
	provider selection.ProviderPayload,
	sourcePath string,
	isFile bool,
	target config.WorkflowTarget,
) error {
	workflowConfiguration := s.workflowConfiguration()
	syncer := s.strmSyncer()
	if syncer == nil {
		return s.fail(ctx, &job, "transferred", "strm_unconfigured", "内置 STRM 同步未配置", true, "transferred")
	}
	job.State = "submitting_sync"
	job.ErrorCode = ""
	job.ErrorMessage = ""
	if err := s.save(ctx, &job, "transferred", "开始写入 STRM"); err != nil {
		return err
	}
	result, err := syncer.Sync(ctx, strm.Request{
		FileID:        provider.FileID,
		SourcePath:    sourcePath,
		TargetPath:    target.QMediaSyncTargetPath,
		IsFile:        isFile,
		Prune:         true,
		StrmBaseURL:   workflowConfiguration.StrmBaseURL,
		StrmRootMount: workflowConfiguration.StrmRootMount,
	})
	if err != nil {
		code, message := builtinSyncFailure(err)
		return s.fail(ctx, &job, "submitting_sync", code, message, true, "transferred")
	}
	job.State = "refreshing_emby"
	job.Attempts = 0
	job.NextAttemptAt = 0
	job.Retryable = false
	return s.save(ctx, &job, "submitting_sync", "STRM 同步完成（"+result.Summary()+"）")
}

func builtinSyncFailure(err error) (string, string) {
	switch {
	case errors.Is(err, strm.ErrAuthExpired):
		return "strm_auth_expired", "115 授权已失效，请在概览页重新扫码后再重试任务"
	case errors.Is(err, strm.ErrPathUnwritable):
		return "strm_path_unwritable", "STRM 目录不可写，请检查 Media Hub 的媒体库挂载"
	case errors.Is(err, strm.ErrNoVideos):
		return "offline_incomplete", "115 离线未完成，目录里还没有视频"
	case errors.Is(err, strm.ErrInvalidRequest):
		return "strm_list_failed", "STRM 同步参数不完整"
	default:
		return "strm_list_failed", "无法列出 115 文件并写入 STRM"
	}
}

func (s *Service) pollSync(ctx context.Context, job store.TransferJob) error {
	provider, err := s.codec.DecodeProvider(job.ProviderToken)
	if err != nil || provider.FileID == "" {
		return s.fail(ctx, &job, "syncing", "provider_state_invalid", "资源转存结果无法解密", false, "")
	}
	target, ok := s.workflowConfiguration().Target(job.MediaType)
	if !ok {
		return s.fail(ctx, &job, "syncing", "strm_unconfigured", "内置 STRM 同步未配置", true, "transferred")
	}
	return s.submitBuiltinSync(ctx, job, provider, provider.Path, provider.IsFile, target)
}

func (s *Service) refreshEmby(ctx context.Context, job store.TransferJob) error {
	target, ok := s.workflowConfiguration().Target(job.MediaType)
	if !ok {
		return s.fail(ctx, &job, "refreshing_emby", "emby_target_unconfigured", "Emby 媒体库未配置", true, "refreshing_emby")
	}
	job.Attempts++
	job.NextAttemptAt = 0
	if err := s.save(ctx, &job, "refreshing_emby", ""); err != nil {
		return err
	}
	if err := s.emby.RefreshLibrary(ctx, target.EmbyLibraryID); err != nil {
		return s.retryExternalStage(ctx, &job, "refreshing_emby", "emby_refresh_failed", "Emby 媒体库刷新失败")
	}
	job.State = "indexing_emby"
	job.Attempts = 0
	job.NextAttemptAt = s.now().UTC().Add(syncPollDelay).Unix()
	job.ErrorCode = ""
	job.ErrorMessage = ""
	return s.save(ctx, &job, "refreshing_emby", "Emby 媒体库已刷新")
}

func (s *Service) pollEmbyIndex(ctx context.Context, job store.TransferJob) error {
	item, found, err := s.findIndexedLibraryItem(ctx, job)
	if err != nil {
		slog.Default().Warn("emby index lookup failed", "job_id", job.ID, "error", err)
		job.Attempts++
		return s.retryExternalStage(ctx, &job, "indexing_emby", "emby_index_unavailable", "无法读取 Emby 入库状态")
	}
	job.Attempts = 0
	if !found {
		if s.now().UTC().Sub(time.Unix(job.CreatedAt, 0).UTC()) > 24*time.Hour {
			return s.fail(ctx, &job, "indexing_emby", "emby_item_missing", "Emby 未找到对应媒体", true, "indexing_emby")
		}
		job.NextAttemptAt = s.now().UTC().Add(syncPollDelay).Unix()
		return s.save(ctx, &job, "indexing_emby", "")
	}
	job.EmbyItemID = item.ID
	if emby.NeedsTMDBIdentify(item, job.TMDBID) {
		if err := s.emby.ApplyTMDBMetadata(ctx, item.ID, job.MediaType, job.Title, job.Year, job.TMDBID, true); err != nil {
			slog.Default().Warn("emby metadata apply failed", "job_id", job.ID, "error", err)
			if saveErr := s.save(ctx, &job, "indexing_emby", "Emby 元数据识别失败，继续完成入库"); saveErr != nil {
				return saveErr
			}
		} else if saveErr := s.save(ctx, &job, "indexing_emby", "已触发 Emby 元数据识别"); saveErr != nil {
			return saveErr
		}
	}
	s.attachLibrarySubtitles(ctx, &job)
	job.State = "verifying_playback"
	job.NextAttemptAt = 0
	return s.save(ctx, &job, "indexing_emby", "Emby 已完成入库")
}

func (s *Service) attachLibrarySubtitles(ctx context.Context, job *store.TransferJob) {
	attacher := s.subtitleAttacher()
	if attacher == nil {
		_ = s.save(ctx, job, "indexing_emby", "自动挂载中文字幕未接入，已跳过")
		return
	}
	if strings.TrimSpace(job.EmbyItemID) == "" {
		_ = s.save(ctx, job, "indexing_emby", "未关联 Emby 条目，跳过字幕挂载")
		return
	}
	ids := []string{job.EmbyItemID}
	if job.MediaType == "series" && s.emby != nil {
		episodes, err := s.emby.Episodes(ctx, job.EmbyItemID)
		if err != nil {
			_ = s.save(ctx, job, "indexing_emby", "无法读取剧集列表，跳过字幕挂载："+sanitizeSubtitleError(err))
			return
		}
		if len(episodes) == 0 {
			_ = s.save(ctx, job, "indexing_emby", "剧集没有分集，跳过字幕挂载")
			return
		}
		ids = make([]string, 0, len(episodes))
		for _, episode := range episodes {
			if episode.ID != "" {
				ids = append(ids, episode.ID)
			}
		}
	}
	attachCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	downloaded := 0
	existed := 0
	missing := 0
	failed := 0
	firstErr := ""
	for _, id := range ids {
		status, err := attacher.AttachChinese(attachCtx, id)
		if err != nil {
			failed++
			if firstErr == "" {
				firstErr = sanitizeSubtitleError(err)
			}
			slog.Default().Warn("attach chinese subtitle failed", "job_id", job.ID, "item_id", id, "error", err)
			continue
		}
		switch status {
		case "attached":
			downloaded++
		case "missing":
			missing++
		case "existed":
			existed++
		}
	}
	switch {
	case downloaded > 0:
		_ = s.save(ctx, job, "indexing_emby", fmt.Sprintf("已自动挂载中文字幕（%d）", downloaded))
	case failed > 0:
		_ = s.save(ctx, job, "indexing_emby", "自动挂载中文字幕失败："+firstErr)
	case missing > 0:
		_ = s.save(ctx, job, "indexing_emby", "未找到可用中文字幕，可稍后在媒体库补")
	case existed > 0:
		_ = s.save(ctx, job, "indexing_emby", "已有中文字幕，未重复下载")
	}
}

func sanitizeSubtitleError(err error) string {
	if err == nil {
		return "未知错误"
	}
	message := strings.TrimSpace(err.Error())
	message = strings.Join(strings.Fields(message), " ")
	if message == "" {
		return "未知错误"
	}
	fields := strings.Fields(message)
	kept := fields[:0]
	for _, field := range fields {
		if strings.Contains(field, "://") || strings.Contains(field, "token=") || strings.Contains(field, "pickcode=") {
			continue
		}
		kept = append(kept, field)
	}
	if len(kept) == 0 {
		return "上游请求失败"
	}
	message = strings.Join(kept, " ")
	runes := []rune(message)
	if len(runes) > 80 {
		return string(runes[:80])
	}
	return message
}

func (s *Service) findIndexedLibraryItem(ctx context.Context, job store.TransferJob) (emby.Item, bool, error) {
	if job.MediaType == "series" {
		return s.emby.FindPlayableItem(
			ctx, job.Title, job.MediaType, job.Year, job.TMDBID, job.Season, job.EpisodeStart, job.EpisodeEnd,
		)
	}
	return s.emby.FindIndexedItem(ctx, job.Title, job.MediaType, job.Year, job.TMDBID)
}

func (s *Service) verifyPlayback(ctx context.Context, job store.TransferJob) error {
	ready, err := s.emby.PlaybackReady(ctx, job.EmbyItemID)
	if err != nil {
		job.Attempts++
		return s.retryExternalStage(ctx, &job, "verifying_playback", "playback_check_failed", "无法读取播放信息")
	}
	job.Attempts = 0
	if !ready {
		if s.now().UTC().Sub(time.Unix(job.CreatedAt, 0).UTC()) > 24*time.Hour {
			return s.fail(ctx, &job, "verifying_playback", "playback_not_ready", "媒体尚未具备播放信息", true, "verifying_playback")
		}
		job.NextAttemptAt = s.now().UTC().Add(syncPollDelay).Unix()
		return s.save(ctx, &job, "verifying_playback", "")
	}
	job.State = "completed"
	job.NextAttemptAt = 0
	job.ErrorCode = ""
	job.ErrorMessage = ""
	return s.save(ctx, &job, "verifying_playback", "播放信息已就绪")
}

func (s *Service) retryExternalStage(
	ctx context.Context,
	job *store.TransferJob,
	state, code, message string,
) error {
	if job.Attempts < maxAutomaticTries {
		job.ErrorCode = code
		job.ErrorMessage = message
		job.NextAttemptAt = s.now().UTC().Add(backoff(job.Attempts)).Unix()
		return s.save(ctx, job, state, "等待自动重试")
	}
	return s.fail(ctx, job, state, code, message, true, state)
}

func (s *Service) fail(
	ctx context.Context,
	job *store.TransferJob,
	expectedState, code, message string,
	retryable bool,
	resumeState string,
) error {
	job.State = "failed"
	job.ResumeState = resumeState
	job.ErrorCode = code
	job.ErrorMessage = message
	job.Retryable = retryable
	job.NextAttemptAt = 0
	return s.save(ctx, job, expectedState, message)
}

func (s *Service) save(ctx context.Context, job *store.TransferJob, expectedState, eventMessage string) error {
	job.UpdatedAt = s.now().UTC().Unix()
	_, err := s.store.UpdateTransferJob(ctx, *job, expectedState, eventMessage)
	return err
}

func backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := 5 * time.Second * time.Duration(1<<(attempt-1))
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}
