package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"media-hub/backend/internal/strm"
)

type strmStub struct {
	status   strm.Status
	enqueue  error
	location string
	redirect error
	input    strm.LibrarySyncInput
}

func (s *strmStub) Status(context.Context) (strm.Status, error) { return s.status, nil }
func (s *strmStub) EnqueueLibrarySync(input strm.LibrarySyncInput) error {
	s.input = input
	return s.enqueue
}
func (s *strmStub) Redirect(context.Context, string, string, string) (string, error) {
	return s.location, s.redirect
}

func TestSTRMStatusRequiresAuth(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewRouter("test", Dependencies{Auth: authStub{}, STRM: &strmStub{status: strm.Status{Mode: "builtin"}}}).
		ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/integrations/strm/status", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", recorder.Code)
	}
}

func TestSTRMStatusAndSync(t *testing.T) {
	service := &strmStub{status: strm.Status{Mode: "builtin", SessionOK: true, MountWritable: true}}
	router := NewRouter("test", Dependencies{Auth: authStub{}, STRM: service})

	statusRecorder := httptest.NewRecorder()
	router.ServeHTTP(statusRecorder, authenticatedRequest(http.MethodGet, "/api/v1/integrations/strm/status"))
	if statusRecorder.Code != http.StatusOK || !strings.Contains(statusRecorder.Body.String(), `"mode":"builtin"`) {
		t.Fatalf("status=%d body=%s", statusRecorder.Code, statusRecorder.Body.String())
	}

	syncRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/integrations/strm/sync", strings.NewReader(`{"mediaType":"all","full":true}`))
	request.Header.Set("Authorization", "Bearer valid-session")
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(syncRecorder, request)
	if syncRecorder.Code != http.StatusAccepted || service.input.MediaType != "all" || !service.input.Full {
		t.Fatalf("status=%d input=%#v body=%s", syncRecorder.Code, service.input, syncRecorder.Body.String())
	}
}

func TestSTRMRedirectIsPublicAndDoesNotProxyBytes(t *testing.T) {
	service := &strmStub{location: "https://cdn.example/video.mkv"}
	recorder := httptest.NewRecorder()
	NewRouter("test", Dependencies{Auth: authStub{}, STRM: service}).
		ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/115/url/video.mkv?pickcode=abc&userid=1", nil))
	if recorder.Code != http.StatusFound {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if location := recorder.Header().Get("Location"); location != service.location {
		t.Fatalf("location=%q", location)
	}
	if strings.Contains(recorder.Body.String(), "cdn.example") && recorder.Code != http.StatusFound {
		t.Fatal("redirect should not proxy media bytes")
	}
}

func TestSTRMRedirectRejectsMissingPickCode(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewRouter("test", Dependencies{Auth: authStub{}, STRM: &strmStub{}}).
		ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/115/url/video.mkv", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recorder.Code)
	}
	var response problem
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(recorder.Body.String(), "pickcode=") {
		t.Fatal("pickcode should not appear in error body")
	}
}
