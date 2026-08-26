package httpapi

import (
	"context"
	"io"
	"net/http"
	"time"

	"media-hub/backend/internal/androidrelease"
	"media-hub/backend/internal/archive"
	"media-hub/backend/internal/auth"
	"media-hub/backend/internal/drive115"
	"media-hub/backend/internal/emby"
	"media-hub/backend/internal/integration"
	"media-hub/backend/internal/localupload"
	"media-hub/backend/internal/playback"
	"media-hub/backend/internal/search"
	"media-hub/backend/internal/settings"
	"media-hub/backend/internal/statistics"
	"media-hub/backend/internal/strm"
	"media-hub/backend/internal/subscription"
	"media-hub/backend/internal/tmdb"
	"media-hub/backend/internal/workflow"
)

type AndroidReleaseProvider interface {
	Latest(context.Context) (androidrelease.Release, error)
	OpenAPK(context.Context, int) (androidrelease.Release, io.ReadSeekCloser, error)
}

type OverviewProvider interface {
	Overview(context.Context) []integration.Health
}

type SearchProvider interface {
	Search(context.Context, string) search.Response
}

type DiscoveryProvider interface {
	Trending(context.Context, string, int) ([]tmdb.DiscoveryItem, error)
	Recommendations(context.Context, string, string, int) ([]tmdb.DiscoveryItem, error)
	Catalog(context.Context, string, string, string, int) ([]tmdb.DiscoveryItem, error)
	Genres(context.Context, string) ([]tmdb.Genre, error)
}

type StatisticsProvider interface {
	Summary(context.Context, int64) (statistics.Summary, error)
}

type Authenticator interface {
	Configured(context.Context) (bool, error)
	Login(context.Context, string, string) (auth.SessionToken, error)
	Authenticate(context.Context, string) (auth.Principal, error)
	ValidateCSRF(auth.Principal, string) error
	Logout(context.Context, auth.Principal) error
	ChangePassword(context.Context, auth.Principal, string, string) error
}

type EmbyReader interface {
	Libraries(context.Context) ([]emby.Library, error)
	SearchItems(context.Context, string, int) (emby.SearchResult, error)
	BrowseItems(context.Context, string, int, int) (emby.SearchResult, error)
	ItemDetails(context.Context, string) (emby.ItemDetail, error)
	RefreshLibrary(context.Context, string) error
	RefreshItem(context.Context, string) error
	DeletePreview(context.Context, string) (emby.DeletePreview, error)
	DeleteItem(context.Context, string) error
	Episodes(context.Context, string) ([]emby.Episode, error)
	PrimaryImage(context.Context, string, int) (emby.PrimaryImage, error)
	SearchRemoteSubtitles(context.Context, string, string) ([]emby.RemoteSubtitle, error)
	DownloadRemoteSubtitle(context.Context, string, string) error
}

type RemoteSubtitles interface {
	Search(context.Context, string, string) ([]emby.RemoteSubtitle, error)
	Download(context.Context, string, string) error
	Local(context.Context, string) (strm.Sidecar, error)
	RemoveLocal(context.Context, string) error
}

type EmbyPosterCache interface {
	Get(itemID string, maxWidth int) (emby.PrimaryImage, bool)
	Put(itemID string, maxWidth int, image emby.PrimaryImage) error
}

type Drive115Reader interface {
	Status(context.Context) (drive115.Status, error)
	ListFiles(context.Context, string, int, int) ([]drive115.FileItem, int, error)
}

type Drive115Authorizer interface {
	Start(context.Context, int64) (drive115.DeviceAuthorization, error)
	Poll(context.Context, int64, string) (drive115.DeviceAuthorization, error)
}

type Drive115CommandService interface {
	Create(context.Context, int64, string, drive115.CommandInput) (drive115.Command, error)
	List(context.Context, int64, int) ([]drive115.Command, error)
	Get(context.Context, int64, string) (drive115.Command, error)
	Confirm(context.Context, int64, string, string) (drive115.Command, error)
	Retry(context.Context, int64, string, string) (drive115.Command, error)
}

type PlaybackService interface {
	CreateDrive115(context.Context, playback.Drive115Target) (playback.Descriptor, error)
	CreateEmbyItem(context.Context, int64, playback.EmbyItemTarget) (playback.Descriptor, error)
	Report(context.Context, int64, string, playback.SessionEvent) error
}

type ArchiveService interface {
	Preview(context.Context, string) ([]archive.Suggestion, error)
	Create(context.Context, int64, []archive.Step) (archive.Plan, error)
	List(context.Context, int64, int) ([]archive.Plan, error)
	Get(context.Context, int64, string) (archive.Plan, error)
	Confirm(context.Context, int64, string, string) (archive.Plan, error)
	Retry(context.Context, int64, string, string) (archive.Plan, error)
}

type LocalUploadService interface {
	Roots() []localupload.Root
	List(string, string) ([]localupload.Entry, error)
	Create(context.Context, int64, string, string, string, string) (localupload.Job, error)
	ListJobs(context.Context, int64, int) ([]localupload.Job, error)
	Get(context.Context, int64, string) (localupload.Job, error)
	Retry(context.Context, int64, string, string) (localupload.Job, error)
}

type TransferWorkflow interface {
	SelectionToken(search.Candidate) string
	Enqueue(context.Context, int64, string, string) (workflow.Job, bool, error)
	Get(context.Context, int64, string) (workflow.JobDetail, error)
	List(context.Context, int64, int, bool) ([]workflow.Job, error)
	SetArchived(context.Context, int64, string, bool) (workflow.Job, error)
	Delete(context.Context, int64, string) error
	Retry(context.Context, int64, string) (workflow.Job, error)
	ListNotifications(context.Context, int64, int) ([]workflow.Notification, error)
	RetryNotification(context.Context, int64, string, string, string) (workflow.Notification, error)
}

type SubscriptionManager interface {
	Create(context.Context, int64, subscription.CreateInput) (subscription.Subscription, error)
	Update(context.Context, int64, string, subscription.UpdateInput) (subscription.Subscription, error)
	Get(context.Context, int64, string) (subscription.Subscription, error)
	List(context.Context, int64) ([]subscription.Subscription, error)
	Export(context.Context, int64) (subscription.Backup, error)
	Import(context.Context, int64, subscription.Backup) (subscription.ImportResult, error)
	SetBatchEnabled(context.Context, int64, subscription.BatchEnabledInput) ([]subscription.Subscription, error)
	Delete(context.Context, int64, string) error
	SetEnabled(context.Context, int64, string, bool) (subscription.Subscription, error)
	RunNow(context.Context, int64, string) (subscription.Run, error)
	Runs(context.Context, int64, string, int) ([]subscription.Run, error)
}

type WeComNotificationTester interface {
	Configured() bool
	Send(context.Context, string) (bool, error)
}

type STRMService interface {
	Status(context.Context) (strm.Status, error)
	EnqueueLibrarySync(strm.LibrarySyncInput) error
	Redirect(context.Context, string, string, string) (string, error)
}

type ProviderSettings interface {
	Get(context.Context, int64) (settings.View, error)
	Update(context.Context, int64, settings.Update) (settings.View, error)
}

type Dependencies struct {
	Auth             Authenticator
	Overview         OverviewProvider
	Search           SearchProvider
	Discovery        DiscoveryProvider
	Statistics       StatisticsProvider
	Emby             EmbyReader
	RemoteSubtitles  RemoteSubtitles
	EmbyPosterCache  EmbyPosterCache
	Drive115         Drive115Reader
	Drive115Auth     Drive115Authorizer
	Drive115Commands Drive115CommandService
	Playback         PlaybackService
	LocalUploads     LocalUploadService
	Archive          ArchiveService
	Workflow         TransferWorkflow
	Subscriptions    SubscriptionManager
	Settings         ProviderSettings
	WeComTester      WeComNotificationTester
	STRM             STRMService
	SecureCookies    bool
	AndroidReleases  AndroidReleaseProvider
	SourceCheckIns   SourceCheckInService
	Web              http.Handler
}

type handler struct {
	version       string
	now           func() time.Time
	dependencies  Dependencies
	loginAttempts *loginLimiter
}

type healthResponse struct {
	Status  string    `json:"status"`
	Version string    `json:"version"`
	Time    time.Time `json:"time"`
}

type systemOverview struct {
	Integrations []integration.Health `json:"integrations"`
}

func NewRouter(version string, dependencies Dependencies) http.Handler {
	h := &handler{
		version: version, now: time.Now,
		dependencies: dependencies, loginAttempts: newLoginLimiter(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", h.getHealth)
	mux.HandleFunc("GET /api/v1/auth/configuration", h.getAuthConfiguration)
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.Handle("GET /api/v1/auth/session", h.protected(h.getSession))
	mux.Handle("POST /api/v1/auth/logout", h.protected(h.logout))
	mux.Handle("PUT /api/v1/auth/password", h.protected(h.changePassword))
	mux.Handle("GET /api/v1/system/overview", h.protected(h.getSystemOverview))
	mux.Handle("GET /api/v1/settings/providers", h.protected(h.getProviderSettings))
	mux.Handle("PUT /api/v1/settings/providers", h.protected(h.updateProviderSettings))
	mux.Handle("GET /api/v1/search", h.protected(h.search))
	mux.Handle("GET /api/v1/discovery/trending", h.protected(h.getTrending))
	mux.Handle("GET /api/v1/discovery/catalog", h.protected(h.getDiscoveryCatalog))
	mux.Handle("GET /api/v1/discovery/genres", h.protected(h.getDiscoveryGenres))
	mux.Handle("GET /api/v1/discovery/{mediaType}/{tmdbId}/recommendations", h.protected(h.getRecommendations))
	mux.Handle("GET /api/v1/integrations/sources/checkins", h.protected(h.listSourceCheckIns))
	mux.Handle("POST /api/v1/integrations/sources/checkins/{id}/retry", h.protected(h.retrySourceCheckIn))
	mux.Handle("GET /api/v1/integrations/strm/status", h.protected(h.getSTRMStatus))
	mux.Handle("POST /api/v1/integrations/strm/sync", h.protected(h.syncSTRMLibrary))
	mux.HandleFunc("GET /115/url/{name}", h.redirectSTRM)
	mux.Handle("GET /api/v1/integrations/emby/libraries", h.protected(h.getEmbyLibraries))
	mux.Handle("GET /api/v1/integrations/emby/items", h.protected(h.searchEmbyItems))
	mux.Handle("GET /api/v1/integrations/emby/libraries/{id}/items", h.protected(h.browseEmbyLibraryItems))
	mux.Handle("POST /api/v1/integrations/emby/libraries/{id}/refresh", h.protected(h.refreshEmbyLibrary))
	mux.Handle("GET /api/v1/integrations/emby/items/{id}", h.protected(h.getEmbyItem))
	mux.Handle("GET /api/v1/integrations/emby/items/{id}/episodes", h.protected(h.getEmbyEpisodes))
	mux.Handle("GET /api/v1/integrations/emby/items/{id}/primary-image", h.protected(h.getEmbyPrimaryImage))
	mux.Handle("POST /api/v1/integrations/emby/items/{id}/refresh", h.protected(h.refreshEmbyItem))
	mux.Handle("GET /api/v1/integrations/emby/items/{id}/remote-subtitles", h.protected(h.searchEmbyRemoteSubtitles))
	mux.Handle("POST /api/v1/integrations/emby/items/{id}/remote-subtitles", h.protected(h.downloadEmbyRemoteSubtitle))
	mux.Handle("GET /api/v1/integrations/emby/items/{id}/local-subtitle", h.protected(h.getLocalSubtitle))
	mux.Handle("GET /api/v1/integrations/emby/items/{id}/delete-preview", h.protected(h.previewEmbyItemDelete))
	mux.Handle("POST /api/v1/integrations/emby/items/{id}/delete", h.protected(h.deleteEmbyItem))
	mux.Handle("POST /api/v1/integrations/wecom/test", h.protected(h.testWeComNotification))
	mux.Handle("POST /api/v1/playback/sessions/{id}/events", h.protected(h.reportPlaybackSession))
	mux.Handle("GET /api/v1/integrations/115/status", h.protected(h.getDrive115Status))
	mux.Handle("POST /api/v1/integrations/115/auth/device", h.protected(h.startDrive115Authorization))
	mux.Handle("GET /api/v1/integrations/115/auth/device/{id}", h.protected(h.pollDrive115Authorization))
	mux.Handle("GET /api/v1/integrations/115/files", h.protected(h.listDrive115Files))
	mux.Handle("POST /api/v1/playback/descriptors", h.protected(h.createDrive115Playback))
	mux.Handle("POST /api/v1/playback/descriptors/drive115", h.protected(h.createDrive115Playback))
	mux.Handle("POST /api/v1/playback/descriptors/emby", h.protected(h.createEmbyPlayback))
	mux.Handle("POST /api/v1/integrations/115/commands", h.protected(h.createDrive115Command))
	mux.Handle("GET /api/v1/integrations/115/commands", h.protected(h.listDrive115Commands))
	mux.Handle("GET /api/v1/integrations/115/commands/{id}", h.protected(h.getDrive115Command))
	mux.Handle("POST /api/v1/integrations/115/commands/{id}/confirm", h.protected(h.confirmDrive115Command))
	mux.Handle("POST /api/v1/integrations/115/commands/{id}/retry", h.protected(h.retryDrive115Command))
	mux.Handle("GET /api/v1/local-uploads/roots", h.protected(h.listLocalUploadRoots))
	mux.Handle("GET /api/v1/local-uploads/files", h.protected(h.listLocalUploadFiles))
	mux.Handle("GET /api/v1/local-uploads", h.protected(h.listLocalUploads))
	mux.Handle("POST /api/v1/local-uploads", h.protected(h.createLocalUpload))
	mux.Handle("GET /api/v1/local-uploads/{id}", h.protected(h.getLocalUpload))
	mux.Handle("POST /api/v1/local-uploads/{id}/retry", h.protected(h.retryLocalUpload))
	mux.Handle("POST /api/v1/archive/preview", h.protected(h.previewArchive))
	mux.Handle("GET /api/v1/archive/plans", h.protected(h.listArchivePlans))
	mux.Handle("POST /api/v1/archive/plans", h.protected(h.createArchivePlan))
	mux.Handle("GET /api/v1/archive/plans/{id}", h.protected(h.getArchivePlan))
	mux.Handle("POST /api/v1/archive/plans/{id}/confirm", h.protected(h.confirmArchivePlan))
	mux.Handle("POST /api/v1/archive/plans/{id}/retry", h.protected(h.retryArchivePlan))
	mux.Handle("GET /api/v1/statistics/summary", h.protected(h.getStatisticsSummary))
	mux.Handle("GET /api/v1/subscriptions", h.protected(h.listSubscriptions))
	mux.Handle("POST /api/v1/subscriptions", h.protected(h.createSubscription))
	mux.Handle("GET /api/v1/subscriptions/backup", h.protected(h.exportSubscriptions))
	mux.Handle("POST /api/v1/subscriptions/backup", h.protected(h.importSubscriptions))
	mux.Handle("POST /api/v1/subscriptions/batch/enabled", h.protected(h.setSubscriptionBatchEnabled))
	mux.Handle("GET /api/v1/subscriptions/{id}", h.protected(h.getSubscription))
	mux.Handle("PUT /api/v1/subscriptions/{id}", h.protected(h.updateSubscription))
	mux.Handle("DELETE /api/v1/subscriptions/{id}", h.protected(h.deleteSubscription))
	mux.Handle("PATCH /api/v1/subscriptions/{id}/enabled", h.protected(h.setSubscriptionEnabled))
	mux.Handle("POST /api/v1/subscriptions/{id}/runs", h.protected(h.runSubscription))
	mux.Handle("GET /api/v1/subscriptions/{id}/runs", h.protected(h.listSubscriptionRuns))
	mux.Handle("GET /api/v1/transfers", h.protected(h.listTransfers))
	mux.Handle("POST /api/v1/transfers", h.protected(h.createTransfer))
	mux.Handle("GET /api/v1/transfers/{id}", h.protected(h.getTransfer))
	mux.Handle("POST /api/v1/transfers/{id}/retry", h.protected(h.retryTransfer))
	mux.Handle("PATCH /api/v1/transfers/{id}/archived", h.protected(h.setTransferArchived))
	mux.Handle("DELETE /api/v1/transfers/{id}", h.protected(h.deleteTransfer))
	mux.Handle("GET /api/v1/notifications", h.protected(h.listNotifications))
	mux.Handle("POST /api/v1/notifications/{jobId}/{eventType}/retry", h.protected(h.retryNotification))
	if dependencies.AndroidReleases != nil {
		mux.Handle("GET /api/v1/client/android/releases/latest", h.protected(h.getLatestAndroidRelease))
		mux.Handle("GET /api/v1/client/android/releases/{versionCode}/apk", h.protected(h.downloadAndroidRelease))
	}
	if dependencies.Web != nil {
		mux.Handle("GET /", dependencies.Web)
	}
	return securityHeaders(mux)
}

func (h *handler) getHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status: "ok", Version: h.version, Time: h.now().UTC(),
	})
}

func (h *handler) getSystemOverview(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Overview == nil {
		writeJSON(w, http.StatusOK, systemOverview{Integrations: []integration.Health{}})
		return
	}
	writeJSON(w, http.StatusOK, systemOverview{
		Integrations: withoutRetiredIntegrations(h.dependencies.Overview.Overview(r.Context())),
	})
}

func withoutRetiredIntegrations(items []integration.Health) []integration.Health {
	visible := make([]integration.Health, 0, len(items))
	for _, item := range items {
		if item.ID == "qmediasync" {
			continue
		}
		visible = append(visible, item)
	}
	return visible
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
