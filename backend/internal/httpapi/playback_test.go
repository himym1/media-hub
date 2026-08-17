package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"media-hub/backend/internal/playback"
)

type playbackStub struct {
	parentID string
	fileID   string
}

func (stub *playbackStub) Create(_ context.Context, parentID, fileID string) (playback.Descriptor, error) {
	stub.parentID = parentID
	stub.fileID = fileID
	return playback.Descriptor{
		StreamURL: "https://cdn.example/video.mkv?token=short",
		UserAgent: playback.PlayerUserAgent,
		Title:     "Movie.mkv",
	}, nil
}

func TestCreatePlaybackDescriptorUsesAuthenticatedRoute(t *testing.T) {
	provider := &playbackStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/playback/descriptors",
		strings.NewReader(`{"parentId":"10","fileId":"20"}`),
	)
	request.Header.Set("Authorization", "Bearer valid-session")

	NewRouter("test-version", Dependencies{Auth: authStub{}, Playback: provider}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if provider.parentID != "10" || provider.fileID != "20" {
		t.Fatalf("request = %q/%q", provider.parentID, provider.fileID)
	}
	var value struct {
		StreamURL string     `json:"streamUrl"`
		UserAgent string     `json:"userAgent"`
		Title     string     `json:"title"`
		ExpiresAt *time.Time `json:"expiresAt"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if value.StreamURL != "https://cdn.example/video.mkv?token=short" || value.UserAgent == "" || value.Title != "Movie.mkv" || value.ExpiresAt != nil {
		t.Fatalf("descriptor = %#v", value)
	}
}

func TestCreatePlaybackDescriptorRequiresAuthentication(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/playback/descriptors", strings.NewReader(`{"parentId":"10","fileId":"20"}`))
	NewRouter("test-version", Dependencies{Auth: authStub{}, Playback: &playbackStub{}}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
}
