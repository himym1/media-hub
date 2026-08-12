package httpapi

import (
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	loginWindow      = 5 * time.Minute
	maxLoginFailures = 5
)

type loginAttempt struct {
	failures int
	resetAt  time.Time
}

type loginLimiter struct {
	mutex    sync.Mutex
	attempts map[string]loginAttempt
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{attempts: make(map[string]loginAttempt)}
}

func (limiter *loginLimiter) allow(address string, now time.Time) bool {
	key := remoteHost(address)
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()
	attempt := limiter.attempts[key]
	if !attempt.resetAt.IsZero() && !now.Before(attempt.resetAt) {
		delete(limiter.attempts, key)
		return true
	}
	return attempt.failures < maxLoginFailures
}

func (limiter *loginLimiter) fail(address string, now time.Time) {
	key := remoteHost(address)
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()
	attempt := limiter.attempts[key]
	if attempt.resetAt.IsZero() || !now.Before(attempt.resetAt) {
		attempt = loginAttempt{resetAt: now.Add(loginWindow)}
	}
	attempt.failures++
	limiter.attempts[key] = attempt
}

func (limiter *loginLimiter) clear(address string) {
	limiter.mutex.Lock()
	delete(limiter.attempts, remoteHost(address))
	limiter.mutex.Unlock()
}

func remoteHost(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err == nil {
		return host
	}
	return address
}

func (h *handler) rateLimitLogin(w http.ResponseWriter, r *http.Request) bool {
	if h.loginAttempts.allow(r.RemoteAddr, h.now()) {
		return true
	}
	w.Header().Set("Retry-After", "300")
	writeProblem(w, problem{
		Type:  "https://media-hub.local/problems/login-rate-limited",
		Title: "登录尝试过于频繁", Status: http.StatusTooManyRequests,
		Code: "login_rate_limited",
	})
	return false
}
