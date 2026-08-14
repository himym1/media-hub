package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"media-hub/backend/internal/wecom"
)

type notificationTesterStub struct {
	configured bool
	unknown    bool
	err        error
	content    string
}

func (stub *notificationTesterStub) Configured() bool { return stub.configured }
func (stub *notificationTesterStub) Send(_ context.Context, content string) (bool, error) {
	stub.content = content
	return stub.unknown, stub.err
}

func authenticatedWebRequest(method, target string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "web-session"})
	return request
}

func TestWeComNotificationTestRequiresCSRF(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := authenticatedWebRequest(http.MethodPost, "/api/v1/integrations/wecom/test")
	tester := &notificationTesterStub{configured: true}

	NewRouter("test-version", Dependencies{Auth: authStub{}, WeComTester: tester}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden || tester.content != "" {
		t.Fatalf("status=%d content=%q", recorder.Code, tester.content)
	}
}

func TestWeComNotificationTestSendsFixedMessage(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := authenticatedWebRequest(http.MethodPost, "/api/v1/integrations/wecom/test")
	request.Header.Set("X-CSRF-Token", "valid-csrf")
	tester := &notificationTesterStub{configured: true}

	NewRouter("test-version", Dependencies{Auth: authStub{}, WeComTester: tester}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || tester.content == "" {
		t.Fatalf("status=%d content=%q", recorder.Code, tester.content)
	}
	var response notificationTestResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || response.Status != "sent" {
		t.Fatalf("response=%q err=%v", recorder.Body.String(), err)
	}
}

func TestWeComNotificationTestReportsUnknownResult(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := authenticatedWebRequest(http.MethodPost, "/api/v1/integrations/wecom/test")
	request.Header.Set("X-CSRF-Token", "valid-csrf")
	tester := &notificationTesterStub{configured: true, unknown: true, err: wecom.SubmissionError{Unknown: true, Err: wecom.ErrSubmissionUnknown}}

	NewRouter("test-version", Dependencies{Auth: authStub{}, WeComTester: tester}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status=%d", recorder.Code)
	}
	var response problem
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || response.Code != "notification_result_unknown" {
		t.Fatalf("response=%q err=%v", recorder.Body.String(), err)
	}
	if !errors.Is(tester.err, wecom.ErrSubmissionUnknown) {
		t.Fatal("stub error lost unknown result")
	}
}
