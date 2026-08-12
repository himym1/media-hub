package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"media-hub/backend/internal/auth"
)

const sessionCookieName = "media_hub_session"

type principalContextKey struct{}

type authConfigurationResponse struct {
	Configured bool `json:"configured"`
}

type loginRequest struct {
	Password string `json:"password"`
	Client   string `json:"client"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type loginResponse struct {
	Authenticated bool      `json:"authenticated"`
	Client        string    `json:"client"`
	ExpiresAt     time.Time `json:"expiresAt"`
	Token         string    `json:"token,omitempty"`
	CSRFToken     string    `json:"csrfToken,omitempty"`
}

type sessionResponse struct {
	Authenticated bool      `json:"authenticated"`
	Client        string    `json:"client"`
	ExpiresAt     time.Time `json:"expiresAt"`
}

func (h *handler) getAuthConfiguration(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.dependencies.Auth == nil {
		writeJSON(w, http.StatusServiceUnavailable, authConfigurationResponse{Configured: false})
		return
	}
	configured, err := h.dependencies.Auth.Configured(r.Context())
	if err != nil {
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/internal",
			Title: "无法读取认证配置", Status: http.StatusInternalServerError,
			Code: "internal_error",
		})
		return
	}
	writeJSON(w, http.StatusOK, authConfigurationResponse{Configured: configured})
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !h.rateLimitLogin(w, r) {
		return
	}
	if h.dependencies.Auth == nil {
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/auth-unavailable",
			Title: "认证服务不可用", Status: http.StatusServiceUnavailable,
			Code: "auth_unavailable",
		})
		return
	}

	var request loginRequest
	if err := decodeJSON(w, r, &request, 4<<10); err != nil || (request.Client != "web" && request.Client != "android") {
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/invalid-login-request",
			Title: "登录请求格式无效", Status: http.StatusBadRequest,
			Code: "invalid_login_request",
		})
		return
	}

	session, err := h.dependencies.Auth.Login(r.Context(), request.Password, request.Client)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrNotConfigured):
			writeProblem(w, problem{
				Type:  "https://media-hub.local/problems/admin-not-configured",
				Title: "管理员尚未初始化", Status: http.StatusServiceUnavailable,
				Code: "admin_not_configured",
			})
		case errors.Is(err, auth.ErrInvalidCredentials):
			h.loginAttempts.fail(r.RemoteAddr, h.now())
			writeProblem(w, problem{
				Type:  "https://media-hub.local/problems/invalid-credentials",
				Title: "密码错误", Status: http.StatusUnauthorized,
				Code: "invalid_credentials",
			})
		default:
			writeProblem(w, problem{
				Type:  "https://media-hub.local/problems/internal",
				Title: "登录失败", Status: http.StatusInternalServerError,
				Code: "internal_error",
			})
		}
		return
	}

	h.loginAttempts.clear(r.RemoteAddr)
	response := loginResponse{
		Authenticated: true, Client: session.Client, ExpiresAt: session.ExpiresAt,
	}
	if session.Client == "web" {
		h.setSessionCookie(w, session.Token, session.ExpiresAt)
		h.setCSRFCookie(w, session.CSRFToken, session.ExpiresAt)
		response.CSRFToken = session.CSRFToken
	} else {
		response.Token = session.Token
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *handler) getSession(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	writeJSON(w, http.StatusOK, sessionResponse{
		Authenticated: true, Client: principal.Client, ExpiresAt: principal.ExpiresAt,
	})
}

func (h *handler) logout(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	if err := h.dependencies.Auth.Logout(r.Context(), principal); err != nil {
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/internal",
			Title: "退出登录失败", Status: http.StatusInternalServerError,
			Code: "internal_error",
		})
		return
	}
	h.clearSessionCookie(w)
	h.clearCSRFCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) changePassword(w http.ResponseWriter, r *http.Request) {
	var request changePasswordRequest
	if err := decodeJSON(w, r, &request, 4<<10); err != nil {
		return
	}
	err := h.dependencies.Auth.ChangePassword(
		r.Context(), principalFromContext(r.Context()), request.CurrentPassword, request.NewPassword,
	)
	if err == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeProblem(w, problem{Type: "https://media-hub.local/problems/invalid-credentials", Title: "当前密码错误", Status: http.StatusUnauthorized, Code: "invalid_credentials"})
	case errors.Is(err, auth.ErrInvalidPassword):
		writeProblem(w, problem{Type: "https://media-hub.local/problems/invalid-password", Title: "新密码必须为 12 至 1024 字节且不能与当前密码相同", Status: http.StatusUnprocessableEntity, Code: "invalid_password"})
	default:
		writeProblem(w, problem{Type: "https://media-hub.local/problems/internal", Title: "密码修改失败", Status: http.StatusInternalServerError, Code: "internal_error"})
	}
}

func (h *handler) protected(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if h.dependencies.Auth == nil {
			writeProblem(w, problem{
				Type:  "https://media-hub.local/problems/auth-unavailable",
				Title: "认证服务不可用", Status: http.StatusServiceUnavailable,
				Code: "auth_unavailable",
			})
			return
		}

		token := requestSessionToken(r)
		principal, err := h.dependencies.Auth.Authenticate(r.Context(), token)
		if err != nil {
			if errors.Is(err, auth.ErrInvalidSession) {
				h.clearSessionCookie(w)
				writeProblem(w, problem{
					Type:  "https://media-hub.local/problems/authentication-required",
					Title: "需要登录", Status: http.StatusUnauthorized,
					Code: "authentication_required",
				})
			} else {
				writeProblem(w, problem{
					Type:  "https://media-hub.local/problems/internal",
					Title: "认证检查失败", Status: http.StatusInternalServerError,
					Code: "internal_error",
				})
			}
			return
		}

		if principal.Client == "web" && isUnsafeMethod(r.Method) {
			if err := h.dependencies.Auth.ValidateCSRF(principal, r.Header.Get("X-CSRF-Token")); err != nil {
				writeProblem(w, problem{
					Type:  "https://media-hub.local/problems/csrf-rejected",
					Title: "请求校验失败", Status: http.StatusForbidden,
					Code: "csrf_rejected",
				})
				return
			}
		}

		ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
		next(w, r.WithContext(ctx))
	})
}

func requestSessionToken(r *http.Request) string {
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(authorization, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		return cookie.Value
	}
	return ""
}

func principalFromContext(ctx context.Context) auth.Principal {
	principal, _ := ctx.Value(principalContextKey{}).(auth.Principal)
	return principal
}

func isUnsafeMethod(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

func (h *handler) setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: token, Path: "/api/v1",
		Expires: expiresAt, HttpOnly: true, Secure: h.dependencies.SecureCookies,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/api/v1",
		MaxAge: -1, HttpOnly: true, Secure: h.dependencies.SecureCookies,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *handler) setCSRFCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: "media_hub_csrf", Value: token, Path: "/",
		Expires: expiresAt, Secure: h.dependencies.SecureCookies,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *handler) clearCSRFCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: "media_hub_csrf", Value: "", Path: "/",
		MaxAge: -1, Secure: h.dependencies.SecureCookies,
		SameSite: http.SameSiteStrictMode,
	})
}
