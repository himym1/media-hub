package subscription

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"media-hub/backend/internal/search"
	"media-hub/backend/internal/store"
	"media-hub/backend/internal/workflow"
)

const (
	workerInterval      = 5 * time.Second
	maxRunAttempts      = 5
	maxSubscriptionRuns = 100
)

func (s *Service) Start(ctx context.Context) error {
	if s == nil || s.store == nil {
		return ErrUnavailable
	}
	go func() {
		defer close(s.done)
		s.runWorker(ctx)
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

func (s *Service) runWorker(ctx context.Context) {
	ticker := time.NewTicker(workerInterval)
	defer ticker.Stop()
	for {
		_ = s.work(ctx)
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-ticker.C:
		}
	}
}

func (s *Service) work(ctx context.Context) error {
	for {
		runID, err := randomID()
		if err != nil {
			return err
		}
		_, _, found, err := s.store.ClaimDueSubscription(ctx, runID, s.now())
		if err != nil {
			return err
		}
		if !found {
			break
		}
	}
	for {
		run, found, err := s.store.NextRunnableSubscriptionRun(ctx, s.now())
		if err != nil {
			return err
		}
		if !found {
			break
		}
		if err := s.processRun(ctx, run); err != nil {
			return err
		}
	}
	return s.reconcileOne(ctx)
}

func (s *Service) processRun(ctx context.Context, run store.SubscriptionRun) error {
	if run.State == "retry_wait" {
		previous := run.State
		run.State = run.ResumeState
		run.ResumeState = ""
		run.NextAttemptAt = 0
		run.UpdatedAt = s.now().UTC().Unix()
		if err := s.store.UpdateSubscriptionRun(ctx, run, previous); err != nil {
			return err
		}
	}
	if run.State == "queued" {
		previous := run.State
		run.State = "searching"
		run.Message = "正在搜索符合订阅规则的资源"
		run.UpdatedAt = s.now().UTC().Unix()
		if err := s.store.UpdateSubscriptionRun(ctx, run, previous); err != nil {
			return err
		}
	}
	if run.State != "searching" {
		return nil
	}
	item, err := s.store.SubscriptionByID(ctx, run.SubscriptionID)
	if err != nil {
		return s.finishRun(ctx, run, "failed", "subscription_missing", "订阅已不存在", false)
	}
	if s.search == nil || s.workflow == nil {
		return s.finishRun(ctx, run, "failed", "subscription_unavailable", "订阅执行器未配置", false)
	}
	response := s.search.Search(ctx, item.Title+seasonLabel(item.Season))
	if len(response.Results) == 0 && len(response.SourceErrors) > 0 {
		retryable := false
		for _, sourceError := range response.SourceErrors {
			retryable = retryable || sourceError.Retryable
		}
		return s.retryRun(ctx, run, "source_search_failed", "所有资源源搜索失败", retryable)
	}
	preferences, sourceIDs, err := decodeRules(item)
	if err != nil {
		return s.finishRun(ctx, run, "failed", "invalid_rules", "订阅规则无法读取", false)
	}
	skip := map[string]struct{}{}
	var candidate search.Candidate
	for {
		next, found := selectCandidate(response.Results, item, sourceIDs, preferences, skip)
		if !found {
			return s.finishRun(ctx, run, "no_match", "", "没有符合规则且身份已验证的资源", false)
		}
		fingerprint := candidateFingerprint(next)
		seen, err := s.store.HasSubscriptionCandidate(ctx, item.ID, fingerprint)
		if err != nil {
			return err
		}
		if seen {
			skip[fingerprint] = struct{}{}
			continue
		}
		candidate = next
		break
	}
	if item.Policy == "once" {
		if candidate.EpisodeEnd > 0 && candidate.EpisodeEnd <= item.LastEpisode {
			return s.finishRun(ctx, run, "duplicate", "", "目标集范围已经处理", false)
		}
		duplicate, err := s.store.HasTransferForIdentity(
			ctx, item.UserID, item.MediaType, item.TMDBID, item.Season, candidate.EpisodeStart, candidate.EpisodeEnd,
		)
		if err != nil {
			return err
		}
		if duplicate {
			return s.finishRun(ctx, run, "duplicate", "", "已有相同 TMDB 身份、季号和集范围的任务", false)
		}
		if s.library != nil {
			_, exists, checkErr := s.library.FindPlayableItem(
				ctx, item.Title, item.MediaType, item.Year, item.TMDBID, item.Season, candidate.EpisodeStart, candidate.EpisodeEnd,
			)
			if checkErr != nil {
				return s.retryRun(ctx, run, "emby_check_failed", "Emby 查重暂时失败", true)
			}
			if exists {
				if err := s.store.AdvanceSubscriptionEpisode(ctx, item.ID, candidate.EpisodeEnd, s.now()); err != nil {
					return err
				}
				return s.finishRun(ctx, run, "duplicate", "", "Emby 已存在目标媒体或集范围", false)
			}
		}
	}
	fingerprint := candidateFingerprint(candidate)
	token := s.workflow.SelectionToken(candidate)
	if token == "" {
		return s.finishRun(ctx, run, "failed", "transfer_unavailable", "资源或目标暂不支持自动转存", false)
	}
	job, _, err := s.workflow.Enqueue(ctx, item.UserID, token, deterministicIdempotencyKey(run.ID))
	if err != nil {
		retryable := errors.Is(err, workflow.ErrUnavailable)
		return s.retryRun(ctx, run, "transfer_enqueue_failed", "创建转存任务失败", retryable)
	}
	previous := run.State
	run.State = "enqueued"
	run.SourceID = candidate.SourceID
	run.CandidateFingerprint = fingerprint
	run.TransferJobID = job.ID
	run.ErrorCode = ""
	run.Message = "已创建转存任务"
	run.Retryable = false
	run.UpdatedAt = s.now().UTC().Unix()
	return s.store.UpdateSubscriptionRun(ctx, run, previous)
}

func (s *Service) reconcileOne(ctx context.Context) error {
	run, found, err := s.store.NextEnqueuedSubscriptionRun(ctx)
	if err != nil || !found {
		return err
	}
	item, err := s.store.SubscriptionByID(ctx, run.SubscriptionID)
	if err != nil {
		return s.finishRun(ctx, run, "failed", "subscription_missing", "订阅已不存在", false)
	}
	job, err := s.workflow.Get(ctx, item.UserID, run.TransferJobID)
	if err != nil {
		return nil
	}
	switch job.State {
	case "completed":
		if err := s.store.AdvanceSubscriptionEpisode(ctx, item.ID, job.EpisodeEnd, s.now()); err != nil {
			return err
		}
		return s.finishRun(ctx, run, "completed", "", "媒体已入库并通过播放验证", false)
	case "failed":
		return s.finishRun(ctx, run, "failed", job.ErrorCode, job.ErrorMessage, job.Retryable)
	case "needs_attention":
		return s.finishRun(ctx, run, "needs_attention", job.ErrorCode, job.ErrorMessage, false)
	default:
		previous := run.State
		run.UpdatedAt = s.now().UTC().Unix()
		return s.store.UpdateSubscriptionRun(ctx, run, previous)
	}
}

func (s *Service) retryRun(ctx context.Context, run store.SubscriptionRun, code, message string, retryable bool) error {
	previous := run.State
	run.Attempts++
	run.ErrorCode = code
	run.Message = message
	run.Retryable = retryable
	run.UpdatedAt = s.now().UTC().Unix()
	if !retryable || run.Attempts >= maxRunAttempts {
		run.State = "failed"
		run.ResumeState = ""
		run.NextAttemptAt = 0
		run.FinishedAt = run.UpdatedAt
		return s.store.UpdateSubscriptionRun(ctx, run, previous)
	}
	run.State = "retry_wait"
	run.ResumeState = "searching"
	run.NextAttemptAt = s.now().UTC().Add(runBackoff(run.Attempts)).Unix()
	return s.store.UpdateSubscriptionRun(ctx, run, previous)
}

func (s *Service) finishRun(ctx context.Context, run store.SubscriptionRun, state, code, message string, retryable bool) error {
	previous := run.State
	run.State = state
	run.ResumeState = ""
	run.ErrorCode = code
	run.Message = message
	run.Retryable = retryable
	run.NextAttemptAt = 0
	run.FinishedAt = s.now().UTC().Unix()
	run.UpdatedAt = run.FinishedAt
	return s.store.UpdateSubscriptionRun(ctx, run, previous)
}

func runBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > maxRunAttempts {
		attempt = maxRunAttempts
	}
	return time.Duration(1<<(attempt-1)) * time.Minute
}

func decodeRules(item store.Subscription) (Preferences, []string, error) {
	var preferences Preferences
	var sourceIDs []string
	if err := json.Unmarshal([]byte(item.PreferencesJSON), &preferences); err != nil {
		return Preferences{}, nil, err
	}
	if err := json.Unmarshal([]byte(item.SourceIDsJSON), &sourceIDs); err != nil {
		return Preferences{}, nil, err
	}
	return preferences, sourceIDs, nil
}

func selectCandidate(candidates []search.Candidate, item store.Subscription, sourceIDs []string, preferences Preferences, skip map[string]struct{}) (search.Candidate, bool) {
	var selected search.Candidate
	selectedScore := -1
	found := false
	for _, candidate := range candidates {
		if _, skipped := skip[candidateFingerprint(candidate)]; skipped {
			continue
		}
		if !candidate.IdentityVerified || candidate.TMDBID != item.TMDBID || candidate.MediaType != item.MediaType {
			continue
		}
		if item.Year > 0 && candidate.Year != item.Year {
			continue
		}
		if item.Season > 0 && candidate.Season != item.Season {
			continue
		}
		if candidate.EpisodeEnd > 0 && candidate.EpisodeEnd <= item.LastEpisode {
			continue
		}
		if !containsSource(sourceIDs, candidate.SourceID) || !allowedCandidate(candidate, preferences) {
			continue
		}
		score := candidateScore(candidate, preferences)
		if !found || score > selectedScore || (score == selectedScore && preferCandidate(candidate, selected, preferences.PreferSmaller)) {
			selected = candidate
			selectedScore = score
			found = true
		}
	}
	return selected, found
}

func allowedCandidate(candidate search.Candidate, preferences Preferences) bool {
	if !containsFold(preferences.Resolutions, candidate.Release.Resolution) ||
		!containsFold(preferences.VideoCodecs, candidate.Release.VideoCodec) ||
		!containsFold(preferences.DynamicRanges, candidate.Release.DynamicRange) {
		return false
	}
	for _, required := range preferences.AudioContains {
		if !strings.Contains(strings.ToLower(candidate.Release.Audio), strings.ToLower(required)) {
			return false
		}
	}
	if candidate.Release.SizeBytes <= 0 {
		return preferences.AllowUnknownSize || (preferences.MinSizeBytes == 0 && preferences.MaxSizeBytes == 0)
	}
	if preferences.MinSizeBytes > 0 && candidate.Release.SizeBytes < preferences.MinSizeBytes {
		return false
	}
	return preferences.MaxSizeBytes == 0 || candidate.Release.SizeBytes <= preferences.MaxSizeBytes
}

func candidateScore(candidate search.Candidate, preferences Preferences) int {
	score := preferenceScore(preferences.PreferredSources, candidate.SourceID, 10000)
	score += preferenceScore(preferences.Resolutions, candidate.Release.Resolution, 1000)
	score += preferenceScore(preferences.VideoCodecs, candidate.Release.VideoCodec, 100)
	score += preferenceScore(preferences.DynamicRanges, candidate.Release.DynamicRange, 10)
	return score
}

func preferenceScore(values []string, value string, weight int) int {
	for index, candidate := range values {
		if strings.EqualFold(candidate, value) {
			return (len(values) - index) * weight
		}
	}
	return 0
}

func preferCandidate(left, right search.Candidate, preferSmaller bool) bool {
	if preferSmaller && left.Release.SizeBytes > 0 && right.Release.SizeBytes > 0 && left.Release.SizeBytes != right.Release.SizeBytes {
		return left.Release.SizeBytes < right.Release.SizeBytes
	}
	leftKey := left.SourceID + "\x00" + left.ID
	rightKey := right.SourceID + "\x00" + right.ID
	return leftKey < rightKey
}

func containsSource(values []string, value string) bool {
	if len(values) == 0 {
		return true
	}
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func containsFold(values []string, value string) bool {
	if len(values) == 0 {
		return true
	}
	for _, candidate := range values {
		if strings.EqualFold(candidate, value) {
			return true
		}
	}
	return false
}

func candidateFingerprint(candidate search.Candidate) string {
	canonical, _ := json.Marshal(struct {
		SourceID     string              `json:"sourceId"`
		ID           string              `json:"id"`
		TMDBID       string              `json:"tmdbId"`
		Season       int                 `json:"season"`
		EpisodeStart int                 `json:"episodeStart"`
		EpisodeEnd   int                 `json:"episodeEnd"`
		Release      search.ReleaseFacts `json:"release"`
	}{candidate.SourceID, candidate.ID, candidate.TMDBID, candidate.Season, candidate.EpisodeStart, candidate.EpisodeEnd, candidate.Release})
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:])
}
