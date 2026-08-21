package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"media-hub/backend/internal/emby"
	"media-hub/backend/internal/wecom"
)

type notificationTesterStub struct {
	configured bool
	unknown    bool
	err        error
	content    string
	ctxErr     error
}

func (stub *notificationTesterStub) Configured() bool { return stub.configured }
func (stub *notificationTesterStub) Send(ctx context.Context, content string) (bool, error) {
	stub.content = content
	stub.ctxErr = ctx.Err()
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

func TestWeComNotificationTestIncludesRejectedErrorCode(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := authenticatedWebRequest(http.MethodPost, "/api/v1/integrations/wecom/test")
	request.Header.Set("X-CSRF-Token", "valid-csrf")
	tester := &notificationTesterStub{configured: true, err: wecom.SubmissionError{Code: 60020, Err: wecom.ErrRejected}}

	NewRouter("test-version", Dependencies{Auth: authStub{}, WeComTester: tester}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway || !strings.Contains(recorder.Body.String(), "60020") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestDeleteEmbyItemSendsWeComNotification(t *testing.T) {
	provider := &embyStub{}
	tester := &notificationTesterStub{configured: true}
	recorder := httptest.NewRecorder()
	NewRouter("test-version", Dependencies{Auth: authStub{}, Emby: provider, WeComTester: tester}).ServeHTTP(recorder, deleteEmbyItemRequest("item-1"))
	if recorder.Code != http.StatusOK || provider.deletedItem != "item-1" {
		t.Fatalf("status=%d deleted=%q body=%s", recorder.Code, provider.deletedItem, recorder.Body.String())
	}
	if tester.content != "Media Hub\n《Movie》已从片库删除\n115 云盘文件已保留" {
		t.Fatalf("content=%q", tester.content)
	}
}

func TestDeleteEmbyItemNotifiesAfterCanceledRequest(t *testing.T) {
	provider := &embyStub{}
	tester := &notificationTesterStub{configured: true}
	request := deleteEmbyItemRequest("item-1")
	ctx, cancel := context.WithCancel(request.Context())
	cancel()
	recorder := httptest.NewRecorder()
	NewRouter("test-version", Dependencies{Auth: authStub{}, Emby: provider, WeComTester: tester}).ServeHTTP(recorder, request.WithContext(ctx))
	if recorder.Code != http.StatusOK || tester.content == "" || tester.ctxErr != nil {
		t.Fatalf("status=%d content=%q ctxErr=%v", recorder.Code, tester.content, tester.ctxErr)
	}
}

func TestDeleteEmbyItemSucceedsWhenWeComFails(t *testing.T) {
	provider := &embyStub{}
	tester := &notificationTesterStub{configured: true, err: wecom.ErrRejected}
	recorder := httptest.NewRecorder()
	NewRouter("test-version", Dependencies{Auth: authStub{}, Emby: provider, WeComTester: tester}).ServeHTTP(recorder, deleteEmbyItemRequest("item-1"))
	if recorder.Code != http.StatusOK || provider.deletedItem != "item-1" {
		t.Fatalf("status=%d deleted=%q body=%s", recorder.Code, provider.deletedItem, recorder.Body.String())
	}
}

func TestDeleteEmbyItemDoesNotNotifyWhenDeleteFails(t *testing.T) {
	provider := &embyStub{deleteErr: emby.ErrDeleteRejected}
	tester := &notificationTesterStub{configured: true}
	recorder := httptest.NewRecorder()
	NewRouter("test-version", Dependencies{Auth: authStub{}, Emby: provider, WeComTester: tester}).ServeHTTP(recorder, deleteEmbyItemRequest("item-1"))
	if recorder.Code != http.StatusBadGateway || provider.deletedItem != "" || tester.content != "" {
		t.Fatalf("status=%d deleted=%q content=%q", recorder.Code, provider.deletedItem, tester.content)
	}
}

func TestLibraryDeleteNotificationUsesFallbackName(t *testing.T) {
	got := libraryDeleteNotification(emby.DeletePreview{CloudKept: true})
	if got != "Media Hub\n《该条目》已从片库删除\n115 云盘文件已保留" {
		t.Fatalf("message=%q", got)
	}
	versions := libraryDeleteNotification(emby.DeletePreview{Name: "Movie", CloudKept: true, VersionCount: 2})
	if versions != "Media Hub\n《Movie》已从片库删除（2 个版本）\n115 云盘文件已保留" {
		t.Fatalf("versions=%q", versions)
	}
}

func deleteEmbyItemRequest(id string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/integrations/emby/items/"+id+"/delete", strings.NewReader(`{"confirmation":"`+id+`"}`))
	request.Header.Set("Authorization", "Bearer valid-session")
	request.Header.Set("Content-Type", "application/json")
	return request
}
