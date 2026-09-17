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
	"media-hub/backend/internal/emby"
	"media-hub/backend/internal/integration"
	"media-hub/backend/internal/search"
	"media-hub/backend/internal/settings"
	"media-hub/backend/internal/strm"
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
	archivedList   bool
	archivedSet    *bool
}

func (*workflowStub) SelectionToken(search.Candidate) string { return "selection" }
func (stub *workflowStub) Enqueue(_ context.Context, _ int64, _ string, key string) (workflow.Job, bool, error) {
	stub.idempotencyKey = key
	return workflow.Job{ID: "job-1", Title: "Movie", MediaType: "movie", Source: "frame", State: "queued"}, true, nil
}
func (stub *workflowStub) EnqueueShareImport(_ context.Context, _ int64, title, shareCode, receiveCode, key string) (workflow.Job, bool, error) {
	stub.idempotencyKey = key
	if title == "" {
		title = shareCode
	}
	return workflow.Job{ID: "job-share", Title: title, MediaType: "adult", Source: "share", State: "queued", TMDBID: ""}, true, nil
}
func (*workflowStub) Get(context.Context, int64, string) (workflow.JobDetail, error) {
	return workflow.JobDetail{}, nil
}
func (stub *workflowStub) List(_ context.Context, _ int64, _ int, archived bool) ([]workflow.Job, error) {
	stub.archivedList = archived
	return []workflow.Job{}, nil
}
func (stub *workflowStub) SetArchived(_ context.Context, _ int64, _ string, archived bool) (workflow.Job, error) {
	stub.archivedSet = &archived
	return workflow.Job{ID: "job-1", State: "completed", Archived: archived}, nil
}
func (*workflowStub) Delete(context.Context, int64, string) error { return nil }
func (*workflowStub) Retry(context.Context, int64, string) (workflow.Job, error) {
	return workflow.Job{}, nil
}
func (*workflowStub) ListNotifications(context.Context, int64, int) ([]workflow.Notification, error) {
	return []workflow.Notification{}, nil
}
func (*workflowStub) RetryNotification(context.Context, int64, string, string, string) (workflow.Notification, error) {
	return workflow.Notification{}, nil
}

type embyStub struct {
	refreshedLibrary   string
	refreshedItem      string
	deletedItem        string
	deleteErr          error
	downloadedSubtitle string
}

func (*embyStub) Libraries(context.Context) ([]emby.Library, error) {
	return []emby.Library{{ID: "library-1", Name: "Movies", CollectionType: "movies"}}, nil
}
func (*embyStub) SearchItems(context.Context, string, int) (emby.SearchResult, error) {
	return emby.SearchResult{}, nil
}
func (*embyStub) BrowseItems(_ context.Context, libraryID string, offset, limit int) (emby.SearchResult, error) {
	return emby.SearchResult{Items: []emby.Item{{ID: libraryID + "-item", Name: "Movie", Type: "Movie"}}, Total: offset + limit + 1}, nil
}
func (*embyStub) ItemDetails(_ context.Context, itemID string) (emby.ItemDetail, error) {
	return emby.ItemDetail{
		Item:        emby.Item{ID: itemID, Name: "Movie", Type: "Movie"},
		ExternalURL: "https://emby.example/web/index.html#!/item?id=" + itemID,
		AppURL:      "emby://items/server-1/" + itemID,
	}, nil
}
func (*embyStub) Episodes(_ context.Context, seriesID string) ([]emby.Episode, error) {
	return []emby.Episode{{
		Item:        emby.Item{ID: seriesID + "-episode-1", Name: "Episode 1", Type: "Episode", Season: 1, Episode: 1},
		ExternalURL: "https://emby.example/web/index.html#!/item?id=" + seriesID + "-episode-1",
		AppURL:      "emby://items/server-1/" + seriesID + "-episode-1",
	}}, nil
}
func (*embyStub) PrimaryImage(context.Context, string, int) (emby.PrimaryImage, error) {
	return emby.PrimaryImage{Data: []byte("image"), ContentType: "image/jpeg"}, nil
}
func (*embyStub) BackdropImage(context.Context, string, int) (emby.PrimaryImage, error) {
	return emby.PrimaryImage{Data: []byte("backdrop"), ContentType: "image/jpeg"}, nil
}
func (stub *embyStub) RefreshLibrary(_ context.Context, id string) error {
	stub.refreshedLibrary = id
	return nil
}
func (stub *embyStub) RefreshItem(_ context.Context, id string) error {
	stub.refreshedItem = id
	return nil
}
func (stub *embyStub) DeletePreview(_ context.Context, id string) (emby.DeletePreview, error) {
	return emby.DeletePreview{
		ID: id, Name: "Movie", Type: "Movie", FileCount: 1, DeletesFiles: true, CloudKept: true, VersionCount: 1,
	}, nil
}
func (stub *embyStub) DeleteItem(_ context.Context, id string) error {
	if stub.deleteErr != nil {
		return stub.deleteErr
	}
	stub.deletedItem = id
	return nil
}
func (*embyStub) SearchRemoteSubtitles(context.Context, string, string) ([]emby.RemoteSubtitle, error) {
	return []emby.RemoteSubtitle{{
		ID: "opensubtitles-1", Name: "Movie.chi.srt", Language: "chi", Format: "srt", ProviderName: "Open Subtitles",
	}}, nil
}
func (stub *embyStub) DownloadRemoteSubtitle(_ context.Context, itemID, subtitleID string) error {
	stub.downloadedSubtitle = itemID + ":" + subtitleID
	return nil
}

type localSubtitleStub struct {
	sidecar strm.Sidecar
	err     error
}

func (stub localSubtitleStub) Search(context.Context, string, string) ([]emby.RemoteSubtitle, error) {
	return nil, nil
}
func (stub localSubtitleStub) Download(context.Context, string, string) error { return nil }
func (stub localSubtitleStub) Local(context.Context, string) (strm.Sidecar, error) {
	return stub.sidecar, stub.err
}
func (stub localSubtitleStub) RemoveLocal(context.Context, string) error { return nil }

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
			SourceRef: "private", TransferState: "available",
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

func TestSearchOmitsResultsThatCannotTransfer(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodGet, "/api/v1/search?query=%E8%8C%83%E6%B5%B7%E8%BE%9B")
	provider := searchStub{response: search.Response{
		Query: "范海辛", Results: []search.Candidate{
			{ID: "juying:magnet", Title: "范海辛", MediaType: "movie", TransferState: "unavailable"},
			{ID: "juying:share", Title: "范海辛", MediaType: "movie", SourceRef: "private", TransferState: "available"},
		},
	}}

	NewRouter("test-version", Dependencies{Auth: authStub{}, Search: provider, Workflow: &workflowStub{}}).ServeHTTP(recorder, request)

	var response search.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if recorder.Code != http.StatusOK || len(response.Results) != 1 || response.Results[0].ID != "juying:share" {
		t.Fatalf("status=%d results=%#v", recorder.Code, response.Results)
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

func TestCreateShareImportAccepts115URL(t *testing.T) {
	provider := &workflowStub{}
	recorder := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodPost, "/api/v1/share-imports")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "share_one_1")
	request.Body = io.NopCloser(strings.NewReader(`{"url":"https://115.com/s/shareABC123?password=ab12","title":"SSIS-001"}`))
	NewRouter("test-version", Dependencies{Auth: authStub{}, Workflow: provider}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted || provider.idempotencyKey != "share_one_1" {
		t.Fatalf("status=%d key=%q body=%s", recorder.Code, provider.idempotencyKey, recorder.Body.String())
	}
}

func TestCreateShareImportAcceptsVideoURL(t *testing.T) {
	provider := &workflowStub{}
	recorder := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodPost, "/api/v1/share-imports")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "share_url_1")
	request.Body = io.NopCloser(strings.NewReader(`{"url":"https://cdn.example.com/clip.mkv","title":"SSIS-001"}`))
	NewRouter("test-version", Dependencies{Auth: authStub{}, Workflow: provider}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted || provider.idempotencyKey != "share_url_1" {
		t.Fatalf("status=%d key=%q body=%s", recorder.Code, provider.idempotencyKey, recorder.Body.String())
	}
}

func TestCreateShareImportRejectsOtherCloud(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodPost, "/api/v1/share-imports")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "share_bad_1")
	request.Body = io.NopCloser(strings.NewReader(`{"url":"https://pan.quark.cn/s/nope"}`))
	NewRouter("test-version", Dependencies{Auth: authStub{}, Workflow: &workflowStub{}}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recorder.Code)
	}
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

func TestTransferArchiveFilterAndMutationUseWorkflowBoundary(t *testing.T) {
	provider := &workflowStub{}
	listRecorder := httptest.NewRecorder()
	NewRouter("test-version", Dependencies{Auth: authStub{}, Workflow: provider}).ServeHTTP(
		listRecorder, authenticatedRequest(http.MethodGet, "/api/v1/transfers?archived=true"),
	)
	if listRecorder.Code != http.StatusOK || !provider.archivedList {
		t.Fatalf("archive list status=%d archived=%v", listRecorder.Code, provider.archivedList)
	}

	archiveRecorder := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodPatch, "/api/v1/transfers/job-1/archived")
	request.Body = io.NopCloser(strings.NewReader(`{"archived":true}`))
	request.Header.Set("Content-Type", "application/json")
	NewRouter("test-version", Dependencies{Auth: authStub{}, Workflow: provider}).ServeHTTP(archiveRecorder, request)
	if archiveRecorder.Code != http.StatusOK || provider.archivedSet == nil || !*provider.archivedSet {
		t.Fatalf("archive status=%d archived=%v", archiveRecorder.Code, provider.archivedSet)
	}
}

func TestTransferArchiveRequiresExplicitBoolean(t *testing.T) {
	provider := &workflowStub{}
	recorder := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodPatch, "/api/v1/transfers/job-1/archived")
	request.Body = io.NopCloser(strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	NewRouter("test-version", Dependencies{Auth: authStub{}, Workflow: provider}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || provider.archivedSet != nil {
		t.Fatalf("archive status=%d archived=%v", recorder.Code, provider.archivedSet)
	}
}

func TestTransferDeleteUsesWorkflowBoundary(t *testing.T) {
	provider := &workflowStub{}
	recorder := httptest.NewRecorder()
	NewRouter("test-version", Dependencies{Auth: authStub{}, Workflow: provider}).ServeHTTP(
		recorder, authenticatedRequest(http.MethodDelete, "/api/v1/transfers/job-1"),
	)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d", recorder.Code)
	}
}

func TestEmbyBrowseDetailAndRefreshRoutes(t *testing.T) {
	provider := &embyStub{}
	router := NewRouter("test-version", Dependencies{Auth: authStub{}, Emby: provider})

	browseRecorder := httptest.NewRecorder()
	router.ServeHTTP(browseRecorder, authenticatedRequest(http.MethodGet, "/api/v1/integrations/emby/libraries/library-1/items?offset=20&limit=10"))
	if browseRecorder.Code != http.StatusOK || !strings.Contains(browseRecorder.Body.String(), "library-1-item") {
		t.Fatalf("browse status=%d body=%s", browseRecorder.Code, browseRecorder.Body.String())
	}
	detailRecorder := httptest.NewRecorder()
	router.ServeHTTP(detailRecorder, authenticatedRequest(http.MethodGet, "/api/v1/integrations/emby/items/item-1"))
	if detailRecorder.Code != http.StatusOK || !strings.Contains(detailRecorder.Body.String(), "externalUrl") ||
		!strings.Contains(detailRecorder.Body.String(), `"appUrl":"emby://items/server-1/item-1"`) {
		t.Fatalf("detail status=%d body=%s", detailRecorder.Code, detailRecorder.Body.String())
	}
	episodesRecorder := httptest.NewRecorder()
	router.ServeHTTP(episodesRecorder, authenticatedRequest(http.MethodGet, "/api/v1/integrations/emby/items/series-1/episodes"))
	if episodesRecorder.Code != http.StatusOK || !strings.Contains(episodesRecorder.Body.String(), `"appUrl":"emby://items/server-1/series-1-episode-1"`) {
		t.Fatalf("episodes status=%d body=%s", episodesRecorder.Code, episodesRecorder.Body.String())
	}
	imageRecorder := httptest.NewRecorder()
	router.ServeHTTP(imageRecorder, authenticatedRequest(http.MethodGet, "/api/v1/integrations/emby/items/item-1/primary-image"))
	if imageRecorder.Code != http.StatusOK || imageRecorder.Header().Get("Content-Type") != "image/jpeg" || imageRecorder.Body.String() != "image" {
		t.Fatalf("image status=%d type=%q body=%q", imageRecorder.Code, imageRecorder.Header().Get("Content-Type"), imageRecorder.Body.String())
	}
	for _, target := range []string{
		"/api/v1/integrations/emby/libraries/library-1/refresh",
		"/api/v1/integrations/emby/items/item-1/refresh",
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, authenticatedRequest(http.MethodPost, target))
		if recorder.Code != http.StatusAccepted {
			t.Fatalf("refresh %s status=%d", target, recorder.Code)
		}
	}
	if provider.refreshedLibrary != "library-1" || provider.refreshedItem != "item-1" {
		t.Fatalf("refresh library=%q item=%q", provider.refreshedLibrary, provider.refreshedItem)
	}
	previewRecorder := httptest.NewRecorder()
	router.ServeHTTP(previewRecorder, authenticatedRequest(http.MethodGet, "/api/v1/integrations/emby/items/item-1/delete-preview"))
	if previewRecorder.Code != http.StatusOK || !strings.Contains(previewRecorder.Body.String(), `"cloudKept":true`) ||
		strings.Contains(previewRecorder.Body.String(), "/private") {
		t.Fatalf("delete preview status=%d body=%s", previewRecorder.Code, previewRecorder.Body.String())
	}
	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodPost, "/api/v1/integrations/emby/items/item-1/delete", strings.NewReader(`{"confirmation":"item-1"}`))
	deleteRequest.Header.Set("Authorization", "Bearer valid-session")
	deleteRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusOK || !strings.Contains(deleteRecorder.Body.String(), `"deleted"`) || provider.deletedItem != "item-1" {
		t.Fatalf("delete status=%d body=%s deleted=%q", deleteRecorder.Code, deleteRecorder.Body.String(), provider.deletedItem)
	}
	mismatch := httptest.NewRecorder()
	mismatchRequest := httptest.NewRequest(http.MethodPost, "/api/v1/integrations/emby/items/item-1/delete", strings.NewReader(`{"confirmation":"other"}`))
	mismatchRequest.Header.Set("Authorization", "Bearer valid-session")
	mismatchRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(mismatch, mismatchRequest)
	if mismatch.Code != http.StatusBadRequest {
		t.Fatalf("mismatch status=%d", mismatch.Code)
	}
}

func TestEmbyPrimaryImageUsesPosterCache(t *testing.T) {
	cache, err := emby.OpenPrimaryImageCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.Put("item-1", 320, emby.PrimaryImage{Data: []byte("cached"), ContentType: "image/jpeg"}); err != nil {
		t.Fatal(err)
	}
	provider := &embyStub{}
	router := NewRouter("test-version", Dependencies{Auth: authStub{}, Emby: provider, EmbyPosterCache: cache})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authenticatedRequest(http.MethodGet, "/api/v1/integrations/emby/items/item-1/primary-image"))
	if recorder.Code != http.StatusOK || recorder.Header().Get("X-Poster-Cache") != "HIT" || recorder.Body.String() != "cached" {
		t.Fatalf("status=%d cache=%q body=%q", recorder.Code, recorder.Header().Get("X-Poster-Cache"), recorder.Body.String())
	}
}

func TestEmbyBackdropImageUsesPosterCache(t *testing.T) {
	cache, err := emby.OpenPrimaryImageCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.Put("bd_item-1", 1280, emby.PrimaryImage{Data: []byte("cached-bd"), ContentType: "image/jpeg"}); err != nil {
		t.Fatal(err)
	}
	provider := &embyStub{}
	router := NewRouter("test-version", Dependencies{Auth: authStub{}, Emby: provider, EmbyPosterCache: cache})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authenticatedRequest(http.MethodGet, "/api/v1/integrations/emby/items/item-1/backdrop-image"))
	if recorder.Code != http.StatusOK || recorder.Header().Get("X-Poster-Cache") != "HIT" || recorder.Body.String() != "cached-bd" {
		t.Fatalf("status=%d cache=%q body=%q", recorder.Code, recorder.Header().Get("X-Poster-Cache"), recorder.Body.String())
	}
}

func TestEmbyManagementRoutesRejectInvalidIdentifiersAndPages(t *testing.T) {
	provider := &embyStub{}
	router := NewRouter("test-version", Dependencies{Auth: authStub{}, Emby: provider})
	for _, target := range []string{
		"/api/v1/integrations/emby/items/bad.id",
		"/api/v1/integrations/emby/libraries/library-1/items?offset=-1",
		"/api/v1/integrations/emby/libraries/library-1/items?limit=101",
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, authenticatedRequest(http.MethodGet, target))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("target=%s status=%d", target, recorder.Code)
		}
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

func TestLocalSubtitleRouteServesSidecar(t *testing.T) {
	router := NewRouter("test-version", Dependencies{
		Auth: authStub{},
		RemoteSubtitles: localSubtitleStub{sidecar: strm.Sidecar{
			Name: "chi.srt", ContentType: "application/x-subrip", Body: []byte("1\n00:00:01,000 --> 00:00:02,000\n你好\n"),
		}},
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authenticatedRequest(http.MethodGet, "/api/v1/integrations/emby/items/item-1/local-subtitle"))
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "application/x-subrip" ||
		!strings.Contains(recorder.Body.String(), "你好") ||
		!strings.Contains(recorder.Header().Get("Content-Disposition"), "chi.srt") {
		t.Fatalf("status=%d type=%q disp=%q body=%q", recorder.Code, recorder.Header().Get("Content-Type"), recorder.Header().Get("Content-Disposition"), recorder.Body.String())
	}
}

func TestLocalSubtitleRouteNotFound(t *testing.T) {
	router := NewRouter("test-version", Dependencies{
		Auth:            authStub{},
		RemoteSubtitles: localSubtitleStub{err: strm.ErrSidecarNotFound},
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authenticatedRequest(http.MethodGet, "/api/v1/integrations/emby/items/item-1/local-subtitle"))
	if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), "local_subtitle_not_found") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
