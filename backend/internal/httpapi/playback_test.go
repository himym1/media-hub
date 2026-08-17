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
	driveTarget playback.Drive115Target
	embyTarget  playback.EmbyItemTarget
	userID      int64
	sessionID   string
	event       playback.SessionEvent
}

func (stub *playbackStub) CreateDrive115(_ context.Context, target playback.Drive115Target) (playback.Descriptor, error) {
	stub.driveTarget = target
	return playbackTestDescriptor("Movie.mkv"), nil
}

func (stub *playbackStub) CreateEmbyItem(_ context.Context, userID int64, target playback.EmbyItemTarget) (playback.Descriptor, error) {
	stub.userID = userID
	stub.embyTarget = target
	value := playbackTestDescriptor("Movie")
	value.SessionID = strings.Repeat("a", 48)
	return value, nil
}

func (stub *playbackStub) Report(_ context.Context, userID int64, sessionID string, event playback.SessionEvent) error {
	stub.userID = userID
	stub.sessionID = sessionID
	stub.event = event
	return nil
}

func playbackTestDescriptor(title string) playback.Descriptor {
	return playback.Descriptor{
		StreamURL: "https://cdn.example/video.mkv?token=short",
		UserAgent: playback.PlayerUserAgent,
		Title:     title,
	}
}

func TestCreatePlaybackDescriptorsUseAuthenticatedTypedRoutes(t *testing.T) {
	for _, test := range []struct {
		name         string
		path         string
		body         string
		assertTarget func(*testing.T, *playbackStub)
	}{
		{
			name: "drive115", path: "/api/v1/playback/descriptors/drive115", body: `{"parentId":"10","fileId":"20"}`,
			assertTarget: func(t *testing.T, stub *playbackStub) {
				if stub.driveTarget.ParentID != "10" || stub.driveTarget.FileID != "20" {
					t.Fatalf("target = %#v", stub.driveTarget)
				}
			},
		},
		{
			name: "emby", path: "/api/v1/playback/descriptors/emby", body: `{"itemId":"emby-item"}`,
			assertTarget: func(t *testing.T, stub *playbackStub) {
				if stub.embyTarget.ItemID != "emby-item" {
					t.Fatalf("target = %#v", stub.embyTarget)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := &playbackStub{}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			request.Header.Set("Authorization", "Bearer valid-session")
			NewRouter("test-version", Dependencies{Auth: authStub{}, Playback: provider}).ServeHTTP(recorder, request)
			if recorder.Code != http.StatusCreated {
				t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
			}
			test.assertTarget(t, provider)
			var value struct {
				StreamURL string     `json:"streamUrl"`
				UserAgent string     `json:"userAgent"`
				Title     string     `json:"title"`
				ExpiresAt *time.Time `json:"expiresAt"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &value); err != nil {
				t.Fatal(err)
			}
			if value.StreamURL != "https://cdn.example/video.mkv?token=short" || value.UserAgent == "" || value.Title == "" || value.ExpiresAt != nil {
				t.Fatalf("descriptor = %#v", value)
			}
		})
	}
}

func TestCreatePlaybackDescriptorRequiresAuthentication(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/playback/descriptors/emby", strings.NewReader(`{"itemId":"item"}`))
	NewRouter("test-version", Dependencies{Auth: authStub{}, Playback: &playbackStub{}}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestPlaybackSessionEventUsesAuthenticatedOwner(t *testing.T) {
	provider := &playbackStub{}
	recorder := httptest.NewRecorder()
	sessionID := strings.Repeat("a", 48)
	request := httptest.NewRequest(
		http.MethodPost, "/api/v1/playback/sessions/"+sessionID+"/events",
		strings.NewReader(`{"event":"progress","positionMs":12345,"paused":true}`),
	)
	request.Header.Set("Authorization", "Bearer valid-session")
	NewRouter("test-version", Dependencies{Auth: authStub{}, Playback: provider}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if provider.userID != 1 || provider.sessionID != sessionID || provider.event.Type != playback.SessionProgress || provider.event.PositionMS != 12345 || !provider.event.Paused {
		t.Fatalf("user=%d session=%q event=%#v", provider.userID, provider.sessionID, provider.event)
	}
}
