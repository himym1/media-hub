package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"media-hub/backend/internal/checkin"
)

type checkInStub struct {
	status checkin.Status
}

func (s checkInStub) Status(context.Context) (checkin.Status, error) { return s.status, nil }
func (s checkInStub) Retry(_ context.Context, sourceID string) (checkin.Item, error) {
	return checkin.Item{SourceID: sourceID, Label: "帧影", State: "idle", UpdatedAt: time.Unix(1, 0).UTC()}, nil
}

func TestSourceCheckInRoutesRequireAuthentication(t *testing.T) {
	router := NewRouter("test", Dependencies{Auth: authStub{}, SourceCheckIns: checkInStub{}})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/integrations/sources/checkins", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestListAndRetrySourceCheckIns(t *testing.T) {
	router := NewRouter("test", Dependencies{
		Auth: authStub{},
		SourceCheckIns: checkInStub{status: checkin.Status{Items: []checkin.Item{{
			SourceID: "framehdr", Label: "帧影", State: "completed", Message: "今日已签到", UpdatedAt: time.Unix(1, 0).UTC(),
		}}}},
	})
	list := httptest.NewRecorder()
	router.ServeHTTP(list, authenticatedRequest(http.MethodGet, "/api/v1/integrations/sources/checkins"))
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d", list.Code)
	}
	var status checkin.Status
	if err := json.Unmarshal(list.Body.Bytes(), &status); err != nil || len(status.Items) != 1 || status.Items[0].SourceID != "framehdr" {
		t.Fatalf("status=%s err=%v", list.Body.String(), err)
	}
	retry := httptest.NewRecorder()
	router.ServeHTTP(retry, authenticatedRequest(http.MethodPost, "/api/v1/integrations/sources/checkins/framehdr/retry"))
	if retry.Code != http.StatusAccepted {
		t.Fatalf("retry status = %d body=%s", retry.Code, retry.Body.String())
	}
}
