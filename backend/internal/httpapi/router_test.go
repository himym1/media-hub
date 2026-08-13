package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"media-hub/backend/internal/auth"
	"media-hub/backend/internal/integration"
	"media-hub/backend/internal/search"
	"media-hub/backend/internal/settings"
	"media-hub/backend/internal/workflow"
)

type authStub struct{}

func (authStub) Configured(context.Context) (bool, error) { return true, nil }
func (authStub) Login(_ context.Context, password, client string) (auth.SessionToken, error) {
	if password == "" {
		return auth.SessionToken{}, auth.ErrInvalidCredentials
	}
	return auth.SessionToken{
		Token: "valid-session", CSRFToken: "valid-csrf", Client: client,
		ExpiresAt: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	}, nil
}
func (authStub) Authenticate(_ context.Context, token string) (auth.Principal, error) {
	if token == "web-session" {
		return auth.Principal{
			UserID: 1, Client: "web", ExpiresAt: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		}, nil
	}
	if token != "valid-session" {
		return auth.Principal{}, auth.ErrInvalidSession
	}
	return auth.Principal{
		UserID: 1, Client: "android", ExpiresAt: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	}, nil
}
func (authStub) ValidateCSRF(_ auth.Principal, token string) error {
	if token != "valid-csrf" {
		return auth.ErrInvalidCSRF
	}
	return nil
}
func (authStub) Logout(context.Context, auth.Principal) error                         { return nil }
func (authStub) ChangePassword(context.Context, auth.Principal, string, string) error { return nil }

type overviewStub []integration.Health

func (stub overviewStub) Overview(context.Context) []integration.Health { return stub }

type searchStub struct {
	response search.Response
}

func (stub searchStub) Search(context.Context, string) search.Response { return stub.response }

type workflowStub struct {
	idempotencyKey string
}

func (*workflowStub) SelectionToken(search.Candidate) string { return "selection" }
func (stub *workflowStub) Enqueue(_ context.Context, _ int64, _ string, key string) (workflow.Job, bool, error) {
	stub.idempotencyKey = key
	return workflow.Job{ID: "job-1", Title: "Movie", MediaType: "movie", Source: "frame", State: "queued"}, true, nil
}
func (*workflowStub) Get(context.Context, int64, string) (workflow.JobDetail, error) {
	return workflow.JobDetail{}, nil
}
func (*workflowStub) List(context.Context, int64, int) ([]workflow.Job, error) {
	return []workflow.Job{}, nil
}
func (*workflowStub) Retry(context.Context, int64, string) (workflow.Job, error) {
	return workflow.Job{}, nil
}
func (*workflowStub) ListNotifications(context.Context, int64, int) ([]workflow.Notification, error) {
	return []workflow.Notification{}, nil
}
func (*workflowStub) RetryNotification(context.Context, int64, string, string, string) (workflow.Notification, error) {
	return workflow.Notification{}, nil
}

type settingsStub struct {
	input settings.Update
	err   error
}

func (*settingsStub) Get(context.Context, int64) (settings.View, error) {
	return settings.View{QMediaSync: settings.QMediaSyncView{BaseURL: "https://qms.example", APIKey: settings.SecretStatus{Configured: true}}}, nil
}

func (stub *settingsStub) Update(_ context.Context, _ int64, input settings.Update) (settings.View, error) {
	stub.input = input
	if stub.err != nil {
		return settings.View{}, stub.err
	}
	return settings.View{QMediaSync: settings.QMediaSyncView{BaseURL: input.QMediaSync.BaseURL, APIKey: settings.SecretStatus{Configured: input.QMediaSync.APIKey.Value != ""}}}, nil
}

func TestHealthIsPublic(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)

	NewRouter("test-version", Dependencies{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response healthResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "ok" || response.Version != "test-version" {
		t.Fatal("unexpected health response")
	}
}

func TestProtectedRouteRequiresAuthentication(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/system/overview", nil)

	NewRouter("test-version", Dependencies{Auth: authStub{}}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestLoginSetsWebSessionCookie(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/login",
		strings.NewReader(`{"password":"test-value","client":"web"}`),
	)

	NewRouter("test-version", Dependencies{Auth: authStub{}}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var sessionCookie *http.Cookie
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil || !sessionCookie.HttpOnly {
		t.Fatal("HttpOnly session cookie was not set")
	}
}

func TestSystemOverviewUsesProvider(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodGet, "/api/v1/system/overview")
	provider := overviewStub{{
		ID: "emby", Label: "Emby", Status: integration.StatusHealthy, Detail: "连接正常",
	}}

	NewRouter("test-version", Dependencies{Auth: authStub{}, Overview: provider}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response systemOverview
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Integrations) != 1 || response.Integrations[0].Status != integration.StatusHealthy {
		t.Fatal("unexpected overview response")
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodGet, "/api/v1/search")

	NewRouter("test-version", Dependencies{Auth: authStub{}, Search: searchStub{}}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	var response problem
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != "query_required" {
		t.Fatalf("code = %q, want query_required", response.Code)
	}
}

func TestSearchReturnsProviderResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodGet, "/api/v1/search?query=%E8%8C%83%E6%B5%B7%E8%BE%9B")
	provider := searchStub{response: search.Response{
		Query: "范海辛", Results: []search.Candidate{{
			ID: "frame:release-1", Title: "范海辛", MediaType: "movie", Source: "帧影",
			Release: search.ReleaseFacts{Resolution: "2160p", VideoCodec: "HEVC"},
		}}, SourceErrors: []search.SourceError{},
	}}

	NewRouter("test-version", Dependencies{Auth: authStub{}, Search: provider}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response search.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Results) != 1 || response.Results[0].ID != "frame:release-1" {
		t.Fatal("unexpected search response")
	}
}

func TestProviderSettingsUpdateRequiresCSRFAndDoesNotEchoSecret(t *testing.T) {
	provider := &settingsStub{}
	body := `{"qmediaSync":{"baseUrl":"https://qms.example","apiKey":{"value":"private-key"}},"emby":{},"drive115":{},"tmdb":{},"wecom":{"baseUrl":"https://qyapi.weixin.qq.com","corpId":"corp","secret":{"value":"wecom-secret"},"chatId":"chat"},"workflow":{"qMediaSyncAccountId":0,"movie":{},"series":{}},"sources":[]}`

	for _, test := range []struct {
		name string
		csrf string
		want int
	}{{"missing csrf", "", http.StatusForbidden}, {"valid csrf", "valid-csrf", http.StatusOK}} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPut, "/api/v1/settings/providers", strings.NewReader(body))
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "web-session"})
			request.Header.Set("Content-Type", "application/json")
			if test.csrf != "" {
				request.Header.Set("X-CSRF-Token", test.csrf)
			}
			NewRouter("test-version", Dependencies{Auth: authStub{}, Settings: provider}).ServeHTTP(recorder, request)
			if recorder.Code != test.want {
				t.Fatalf("status = %d, want %d", recorder.Code, test.want)
			}
			if test.want == http.StatusOK {
				for _, secret := range []string{"private-key", "wecom-secret"} {
					if strings.Contains(recorder.Body.String(), secret) {
						t.Fatalf("secret %q was echoed in response", secret)
					}
				}
			}
		})
	}
	if provider.input.QMediaSync.APIKey.Value != "private-key" {
		t.Fatal("settings input was not received")
	}
	if provider.input.WeCom == nil || provider.input.WeCom.Secret.Value != "wecom-secret" {
		t.Fatal("WeCom settings input was not received")
	}
}

func TestProviderSettingsUpdateReturnsConflictForActiveProviderOperations(t *testing.T) {
	provider := &settingsStub{err: settings.ErrActiveProviderOperations}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/settings/providers", strings.NewReader(`{"qmediaSync":{},"emby":{},"drive115":{},"tmdb":{},"workflow":{"movie":{},"series":{}},"sources":[]}`))
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "web-session"})
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", "valid-csrf")

	NewRouter("test-version", Dependencies{Auth: authStub{}, Settings: provider}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}
	var response problem
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != "active_provider_operations" {
		t.Fatalf("code = %q, want active_provider_operations", response.Code)
	}
}

func authenticatedRequest(method, target string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("Authorization", "Bearer valid-session")
	return request
}

func TestCreateTransferRequiresCSRFForWebCookie(t *testing.T) {
	provider := &workflowStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/transfers", strings.NewReader(`{"transferToken":"selection"}`))
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "web-session"})
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "request_one")

	NewRouter("test-version", Dependencies{Auth: authStub{}, Workflow: provider}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestCreateTransferAcceptsAndroidBearerAndIdempotencyKey(t *testing.T) {
	provider := &workflowStub{}
	recorder := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodPost, "/api/v1/transfers")
	request.Body = io.NopCloser(strings.NewReader(`{"transferToken":"selection"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "request_one")

	NewRouter("test-version", Dependencies{Auth: authStub{}, Workflow: provider}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
	if provider.idempotencyKey != "request_one" {
		t.Fatalf("idempotency key = %q", provider.idempotencyKey)
	}
}
