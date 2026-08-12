package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesIndexAndSPAFallback(t *testing.T) {
	handler := Handler()
	for _, target := range []string{"/", "/subscriptions"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", target, recorder.Code, http.StatusOK)
		}
		if !strings.Contains(recorder.Body.String(), "Media Hub") {
			t.Fatalf("%s did not return the Web entry point", target)
		}
		if recorder.Header().Get("Cache-Control") != "no-cache" {
			t.Fatalf("%s cache control = %q", target, recorder.Header().Get("Cache-Control"))
		}
	}
}

func TestHandlerDoesNotMaskMissingAssetsOrAPIPaths(t *testing.T) {
	handler := Handler()
	for _, target := range []string{"/assets/missing.js", "/api/v1/missing"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want %d", target, recorder.Code, http.StatusNotFound)
		}
	}
}
