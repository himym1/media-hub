package qms

import (
	"strings"
	"unicode"
)

const (
	CodeSyncFailed      = "sync_failed"
	CodeSyncAuthExpired = "sync_auth_expired"

	messageSyncFailed    = "QMediaSync 同步失败"
	messageAuthExpired   = "QMediaSync 的 115 授权已失效，请到 QMediaSync「网盘账号」重新授权后再重试任务"
	messageAuthMissing   = "QMediaSync 尚未完成 115 授权，请到 QMediaSync「网盘账号」授权"
	messageAuthDegraded  = "QMediaSync 在线，但 115 授权已失效，请到 QMediaSync「网盘账号」重新授权"
	maxPublicReasonRunes = 80
)

func AuthExpired(reason string) bool {
	lower := strings.ToLower(strings.TrimSpace(reason))
	if lower == "" {
		return false
	}
	for _, marker := range []string{
		"授权失效", "授权过期", "授权失败", "重新授权", "登录失效", "登录过期", "未登录",
		"refresh token", "没有刷新token", "没有刷新 token",
	} {
		if strings.Contains(lower, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

func PublicSyncFailure(reason string) (code, message string) {
	if AuthExpired(reason) {
		return CodeSyncAuthExpired, messageAuthExpired
	}
	if sanitized := sanitizeFailReason(reason); sanitized != "" {
		return CodeSyncFailed, messageSyncFailed + "：" + sanitized
	}
	return CodeSyncFailed, messageSyncFailed
}

func publicRecordFailure(reason string) string {
	if strings.TrimSpace(reason) == "" {
		return ""
	}
	_, message := PublicSyncFailure(reason)
	return message
}

func sanitizeFailReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ""
	}
	lower := strings.ToLower(reason)
	for _, banned := range []string{
		"http://", "https://", "cookie", "token=", "api_key", "password", "/volume", "cid=",
	} {
		if strings.Contains(lower, banned) {
			return ""
		}
	}
	for _, item := range []rune(reason) {
		if item == unicode.ReplacementChar {
			return ""
		}
	}
	runes := []rune(reason)
	if len(runes) > maxPublicReasonRunes {
		return string(runes[:maxPublicReasonRunes]) + "…"
	}
	return reason
}

func accountAuthDetail(accounts []cloudAccount) string {
	var saw115, authorized bool
	for _, account := range accounts {
		if account.SourceType != "115" {
			continue
		}
		saw115 = true
		if strings.TrimSpace(account.TokenFailedReason) != "" {
			if AuthExpired(account.TokenFailedReason) {
				return messageAuthDegraded
			}
			if sanitized := sanitizeFailReason(account.TokenFailedReason); sanitized != "" {
				return "QMediaSync 的 115 授权失败：" + sanitized
			}
			return messageAuthDegraded
		}
		if account.HasToken {
			authorized = true
		}
	}
	if saw115 && !authorized {
		return messageAuthMissing
	}
	return ""
}
