package adapter

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"media-hub/backend/internal/search"
)

type juyingCheckInStats struct {
	Status       string `json:"status"`
	Message      string `json:"message"`
	CheckedToday bool   `json:"checked_today"`
}

type juyingCheckInResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Msg     string `json:"msg"`
}

func (s *Juying) CheckIn(ctx context.Context) (search.CheckInResult, error) {
	if s.authMode != "web" {
		return search.CheckInResult{State: "skipped", Message: "开发者 API 不支持签到，请改用网页登录"}, nil
	}
	if s.account == "" || s.secret == "" {
		return search.CheckInResult{State: "skipped", Message: "聚影账号未配置"}, nil
	}
	var stats juyingCheckInStats
	if err := s.webJSON(ctx, http.MethodGet, "/api/app/checkin/stats/", nil, &stats); err != nil {
		return search.CheckInResult{}, err
	}
	if stats.Status != "success" {
		return search.CheckInResult{}, search.Failure{Code: "checkin_failed", Message: publicCheckInMessage(stats.Message, "无法读取聚影签到状态"), Retryable: true}
	}
	if stats.CheckedToday {
		return search.CheckInResult{State: "completed", Message: "今日已签到"}, nil
	}
	var response juyingCheckInResponse
	if err := s.webJSON(ctx, http.MethodPost, "/api/app/checkin/do/", map[string]string{}, &response); err != nil {
		var latest juyingCheckInStats
		if statsErr := s.webJSON(ctx, http.MethodGet, "/api/app/checkin/stats/", nil, &latest); statsErr == nil && latest.CheckedToday {
			return search.CheckInResult{State: "completed", Message: "今日已签到"}, nil
		}
		var failure search.Failure
		if errors.As(err, &failure) && strings.Contains(failure.Message, "已签") {
			return search.CheckInResult{State: "completed", Message: "今日已签到"}, nil
		}
		return search.CheckInResult{}, err
	}
	if response.Status == "success" {
		return search.CheckInResult{State: "completed", Message: "签到成功"}, nil
	}
	combined := firstNonEmptyString(response.Message, response.Msg)
	if strings.Contains(combined, "已签") {
		return search.CheckInResult{State: "completed", Message: "今日已签到"}, nil
	}
	return search.CheckInResult{}, search.Failure{Code: "checkin_failed", Message: publicCheckInMessage(combined, "聚影签到失败"), Retryable: true}
}

func publicCheckInMessage(reason, fallback string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fallback
	}
	lower := strings.ToLower(reason)
	for _, banned := range []string{"http://", "https://", "cookie", "token", "password", "api_key"} {
		if strings.Contains(lower, banned) {
			return fallback
		}
	}
	runes := []rune(reason)
	if len(runes) > 80 {
		return string(runes[:80]) + "…"
	}
	return reason
}
