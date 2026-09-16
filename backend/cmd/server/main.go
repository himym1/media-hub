package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"media-hub/backend/internal/adapter"
	"media-hub/backend/internal/androidrelease"
	"media-hub/backend/internal/archive"
	"media-hub/backend/internal/assrt"
	"media-hub/backend/internal/auth"
	"media-hub/backend/internal/checkin"
	"media-hub/backend/internal/config"
	"media-hub/backend/internal/desktoprelease"
	"media-hub/backend/internal/drive115"
	"media-hub/backend/internal/emby"
	"media-hub/backend/internal/httpapi"
	"media-hub/backend/internal/integration"
	"media-hub/backend/internal/localupload"
	"media-hub/backend/internal/mediaidentity"
	"media-hub/backend/internal/moviepilot"
	"media-hub/backend/internal/playback"
	"media-hub/backend/internal/search"
	"media-hub/backend/internal/securepayload"
	"media-hub/backend/internal/selection"
	"media-hub/backend/internal/settings"
	"media-hub/backend/internal/statistics"
	"media-hub/backend/internal/store"
	"media-hub/backend/internal/strm"
	"media-hub/backend/internal/strm/builtin"
	"media-hub/backend/internal/subscription"
	"media-hub/backend/internal/subtitlecat"
	"media-hub/backend/internal/subtitles"
	"media-hub/backend/internal/tmdb"
	"media-hub/backend/internal/webui"
	"media-hub/backend/internal/wecom"
	"media-hub/backend/internal/workflow"
)

var version = "0.8.0-dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("media hub api stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	configuration, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	startupContext, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStartup()
	dataStore, err := store.Open(startupContext, configuration.DatabasePath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer dataStore.Close()
	if err := dataStore.SyncIntegrations(startupContext, integrationRecords(configuration.Integrations)); err != nil {
		return fmt.Errorf("sync integration configuration: %w", err)
	}

	authService, err := auth.NewService(dataStore)
	if err != nil {
		return fmt.Errorf("initialize authentication: %w", err)
	}
	adminCreated, err := authService.Bootstrap(startupContext, configuration.BootstrapAdminPassword)
	if err != nil {
		return fmt.Errorf("bootstrap administrator: %w", err)
	}
	if err := authService.PurgeExpired(startupContext); err != nil {
		return fmt.Errorf("purge expired sessions: %w", err)
	}
	if adminCreated {
		logger.Info("administrator account initialized")
	}

	selectionCodec, err := selection.NewCodec(configuration.DataEncryptionKey)
	if err != nil {
		return fmt.Errorf("initialize data encryption: %w", err)
	}
	securePayloadCodec, err := securepayload.New(configuration.DataEncryptionKey)
	if err != nil {
		return fmt.Errorf("initialize secure operation payloads: %w", err)
	}
	embyClient := emby.NewConfiguredClient(emby.RuntimeConfig{
		BaseURL: configuration.Emby.BaseURL, APIKey: configuration.Emby.APIKey,
		UserID: configuration.Emby.UserID, Password: configuration.Emby.Password,
		PlaybackBaseURL: configuration.EmbyPlaybackBaseURL,
		MovieLibraryID:  configuration.Workflow.Movie.EmbyLibraryID,
		SeriesLibraryID: configuration.Workflow.Series.EmbyLibraryID,
	}, configuration.ProbeTimeout)
	sharedEmbyClient := emby.NewConfiguredClient(emby.RuntimeConfig{
		BaseURL: configuration.SharedEmby.BaseURL, Username: configuration.SharedEmby.Username,
		Password: configuration.SharedEmby.Password, ProxyURL: parseOptionalHTTPURL(configuration.SharedEmby.ProxyURL), Shared: true,
	}, configuration.SearchTimeout)
	embyHub := emby.NewHub(embyClient, sharedEmbyClient)
	posterCache, err := emby.OpenPrimaryImageCache(filepath.Join(filepath.Dir(configuration.DatabasePath), "poster-cache"))
	if err != nil {
		return fmt.Errorf("open poster cache: %w", err)
	}
	drive115Client := drive115.NewClient("", configuration.ProbeTimeout)
	drive115AuthService := drive115.NewAuthService(
		dataStore, securePayloadCodec, drive115Client, configuration.ProbeTimeout,
	)
	drive115CommandService := drive115.NewCommandService(dataStore, securePayloadCodec)
	playbackService := playback.NewService(drive115AuthService, embyHub)
	playbackService.ConfigurePublicBase(configuration.Workflow.StrmBaseURL)
	if admin, exists, err := dataStore.Admin(startupContext); err != nil {
		return fmt.Errorf("read administrator for 115 authorization: %w", err)
	} else if exists {
		if err := drive115AuthService.Load(startupContext, admin.ID); err != nil {
			return fmt.Errorf("load encrypted 115 authorization: %w", err)
		}
	}
	tmdbClient := tmdb.NewClient(
		configuration.TMDB.BaseURL,
		configuration.TMDB.AccessToken,
		configuration.ProbeTimeout,
	)
	assrtTimeout := 60 * time.Second
	if configuration.SearchTimeout > assrtTimeout {
		assrtTimeout = configuration.SearchTimeout
	}
	assrtClient := assrt.NewClientWithProxy(configuration.Assrt.BaseURL, configuration.Assrt.Token, assrtTimeout, configuration.AssrtFileProxyURL)
	moviePilotTimeout := 45 * time.Second
	if configuration.SearchTimeout > moviePilotTimeout {
		moviePilotTimeout = configuration.SearchTimeout
	}
	moviePilotClient := moviepilot.NewClient(configuration.MoviePilot.BaseURL, configuration.MoviePilot.APIToken, moviePilotTimeout)
	subtitlecatClient := subtitlecat.NewClient(assrtTimeout, configuration.SourceProxyURL)
	wecomTimeout := configuration.ProbeTimeout
	if wecomTimeout < 10*time.Second {
		wecomTimeout = 10 * time.Second
	}
	wecomClient := wecom.NewConfiguredClient(configuration.WeCom, wecomTimeout)
	searchService := search.NewServiceWithIdentity(tmdbClient)
	workflowService := workflow.NewService(
		dataStore, searchService, selectionCodec, embyClient, wecomClient, configuration.Workflow,
		drive115AuthService.FolderPath,
		func(ctx context.Context, fileID, name string) error {
			return drive115AuthService.ExecuteFileCommand(ctx, "rename", map[string]any{
				"fileId": fileID,
				"name":   name,
			})
		},
		func(ctx context.Context, mediaType, folderID string) error {
			if mediaType != "movie" {
				return nil
			}
			items, _, err := drive115AuthService.ListFiles(ctx, folderID, 200, 0)
			if err != nil {
				return fmt.Errorf("无法校验转存内容")
			}
			names := make([]string, 0, len(items))
			for _, item := range items {
				names = append(names, item.Name)
			}
			if mediaidentity.MovieShareLooksLikeSeries(names) {
				return errors.New("转存内容像是电视剧分集，请按剧集重新搜索")
			}
			return nil
		},
	)
	builtinSyncer := builtin.New(drive115AuthService)
	builtinSyncer.UseDownload(drive115AuthService)
	strmCoordinator := strm.NewCoordinator(
		dataStore,
		builtinSyncer,
		drive115AuthService,
		strm.NewRedirectCache(drive115AuthService),
		workflowService.Workflow,
		logger,
	)
	strmCoordinator.UseAlerter(wecomClient)
	strmCoordinator.UseLibraryRefresher(embyClient)
	workflowService.UseSTRMSyncer(strmCoordinator)
	workflowService.UseFolderVideos(builtinSyncer.HasVideos)
	checkinService := checkin.NewService(dataStore, searchService, wecomClient)
	settingsService := settings.NewService(
		dataStore, securePayloadCodec, settings.FromConfig(configuration),
		func(value settings.Values) {
			embyHub.ConfigureLocal(emby.RuntimeConfig{
				BaseURL: value.Emby.BaseURL, APIKey: value.Emby.APIKey,
				UserID: value.Emby.UserID, Password: value.Emby.Password,
				PlaybackBaseURL: configuration.EmbyPlaybackBaseURL,
				MovieLibraryID:  value.Workflow.Movie.EmbyLibraryID,
				SeriesLibraryID: value.Workflow.Series.EmbyLibraryID,
			})
			embyHub.ConfigureShared(emby.RuntimeConfig{
				BaseURL: value.SharedEmby.BaseURL, Username: value.SharedEmby.Username,
				Password: value.SharedEmby.Password, ProxyURL: parseOptionalHTTPURL(value.SharedEmby.ProxyURL), Shared: true,
			})
			tmdbClient.Configure(value.TMDB.BaseURL, value.TMDB.AccessToken)
			assrtClient.Configure(value.Assrt.BaseURL, value.Assrt.Token)
			moviePilotClient.Configure(value.MoviePilot.BaseURL, value.MoviePilot.APIToken)
			wecomClient.ConfigureDelivery(value.WeCom)
			workflowService.Configure(value.Workflow)
			playbackService.ConfigurePublicBase(value.Workflow.StrmBaseURL)
			runtimeSources := searchSourcesFromSettings(value, configuration.SearchTimeout, configuration.FixtureMode, drive115AuthService, configuration.SourceProxyURL, moviePilotClient)
			searchService.Configure(tmdbClient, runtimeSources...)
			normalized := settings.NormalizeCheckIn(value.CheckIn)
			checkinService.Configure(checkin.Schedule{
				Enabled: normalized.Enabled, Hour: normalized.Hour, Minute: normalized.Minute, Sources: normalized.Sources,
			})
		},
	)
	if admin, exists, err := dataStore.Admin(startupContext); err != nil {
		return fmt.Errorf("read administrator for runtime settings: %w", err)
	} else if exists && securePayloadCodec != nil {
		if err := settingsService.Load(startupContext, admin.ID); err != nil {
			return fmt.Errorf("load encrypted runtime settings: %w", err)
		}
		logger.Info("wecom delivery ready", "configured", wecomClient.Configured(), "mode", settingsService.Values().WeCom.DeliveryMode())
	}
	if workflow := workflowService.Workflow(); workflow.UsesBuiltinSync() {
		if err := strm.MountWritable(workflow.StrmRootMount); err != nil {
			logger.Info("builtin strm mount is not writable", "path", workflow.StrmRootMount)
		}
	}
	subscriptionService := subscription.NewService(dataStore, searchService, workflowService, embyClient)
	subtitleService := subtitles.New(embyClient, assrtClient, func() string {
		return settingsService.Values().Workflow.StrmRootMount
	})
	subtitleService.UseSubtitlecat(subtitlecatClient)
	subtitleService.UseLibraryPaths(func() []strm.PathMapping {
		maps, err := strm.ParsePathMap(configuration.LibraryPathMap)
		if err != nil {
			return nil
		}
		return maps
	})
	workflowService.UseSubtitles(subtitleService)
	overview := integration.NewOverviewService(
		searchService,
		drive115AuthService,
		tmdbClient,
		assrtClient,
		moviePilotClient,
		subtitlecatClient,
		wecomClient,
		embyHub,
		embyHub.SharedHealthChecker(),
		emby.NewPlaybackChecker(embyClient),
		checkinService,
		strmCoordinator,
	)
	statisticsService := statistics.NewService(dataStore)
	localUploadService := localupload.NewService(dataStore, securePayloadCodec, configuration.LocalUploadRoots)
	archiveService := archive.NewService(dataStore, securePayloadCodec, drive115AuthService)
	androidReleaseService := androidrelease.NewService(configuration.AndroidReleaseDir)
	desktopReleaseService := desktoprelease.NewService(configuration.AndroidReleaseDir)

	runtimeContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	driveCommandWorker := drive115.NewCommandWorker(dataStore, securePayloadCodec, drive115AuthService, logger)
	driveCommandDone := make(chan error, 1)
	driveCommandWorkerStarted := securePayloadCodec != nil
	if driveCommandWorkerStarted {
		go func() { driveCommandDone <- driveCommandWorker.Run(runtimeContext) }()
	}
	localUploadWorker := localupload.NewWorker(localUploadService, drive115AuthService, logger)
	localUploadDone := make(chan error, 1)
	localUploadWorkerStarted := securePayloadCodec != nil && len(configuration.LocalUploadRoots) > 0
	if localUploadWorkerStarted {
		go func() { localUploadDone <- localUploadWorker.Run(runtimeContext) }()
	}
	archiveWorker := archive.NewWorker(archiveService, logger)
	archiveDone := make(chan error, 1)
	archiveWorkerStarted := securePayloadCodec != nil
	if archiveWorkerStarted {
		go func() { archiveDone <- archiveWorker.Run(runtimeContext) }()
	}
	if err := subscriptionService.Start(runtimeContext); err != nil {
		return fmt.Errorf("start subscription worker: %w", err)
	}
	subscriptionWorkerStarted := true
	if err := checkinService.Start(runtimeContext); err != nil {
		return fmt.Errorf("start source check-in worker: %w", err)
	}
	checkinWorkerStarted := true
	strmCoordinator.Start(runtimeContext)
	workerStarted := false
	if selectionCodec != nil || wecomClient.Configured() {
		if err := workflowService.Start(runtimeContext); err != nil {
			return fmt.Errorf("start transfer worker: %w", err)
		}
		workerStarted = true
	}

	server := &http.Server{
		Addr: configuration.Address,
		Handler: httpapi.NewRouter(version, httpapi.Dependencies{
			Auth: authService, Overview: overview, Search: searchService, Discovery: tmdbClient,
			Emby: embyHub, RemoteSubtitles: subtitleService, EmbyPosterCache: posterCache, Drive115: drive115AuthService, Drive115Auth: drive115AuthService, Drive115Commands: drive115CommandService,
			Playback: playbackService,
			Workflow: workflowService, Subscriptions: subscriptionService, Statistics: statisticsService, LocalUploads: localUploadService, Archive: archiveService, AndroidReleases: androidReleaseService, DesktopReleases: desktopReleaseService, SourceCheckIns: checkinService,
			Settings: settingsService, WeComTester: wecomClient, STRM: strmCoordinator,
			SecureCookies: configuration.SecureCookies,
			Web:           webui.Handler(),
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("media hub api listening", "addr", configuration.Address, "version", version)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		stop()
		if driveCommandWorkerStarted {
			if waitErr := <-driveCommandDone; waitErr != nil {
				return fmt.Errorf("stop 115 command worker: %w", waitErr)
			}
		}
		if localUploadWorkerStarted {
			if waitErr := <-localUploadDone; waitErr != nil {
				return fmt.Errorf("stop local upload worker: %w", waitErr)
			}
		}
		if archiveWorkerStarted {
			if waitErr := <-archiveDone; waitErr != nil {
				return fmt.Errorf("stop archive worker: %w", waitErr)
			}
		}
		if subscriptionWorkerStarted {
			waitContext, cancelWait := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancelWait()
			if waitErr := subscriptionService.Wait(waitContext); waitErr != nil {
				return fmt.Errorf("stop subscription worker: %w", waitErr)
			}
		}
		if checkinWorkerStarted {
			waitContext, cancelWait := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancelWait()
			if waitErr := checkinService.Wait(waitContext); waitErr != nil {
				return fmt.Errorf("stop source check-in worker: %w", waitErr)
			}
		}
		if workerStarted {
			waitContext, cancelWait := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancelWait()
			if waitErr := workflowService.Wait(waitContext); waitErr != nil {
				return fmt.Errorf("stop transfer worker: %w", waitErr)
			}
		}
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-runtimeContext.Done():
		shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()
		if err := server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}
		if driveCommandWorkerStarted {
			select {
			case err := <-driveCommandDone:
				if err != nil {
					return fmt.Errorf("stop 115 command worker: %w", err)
				}
			case <-shutdownContext.Done():
				return fmt.Errorf("stop 115 command worker: %w", shutdownContext.Err())
			}
		}
		if localUploadWorkerStarted {
			select {
			case err := <-localUploadDone:
				if err != nil {
					return fmt.Errorf("stop local upload worker: %w", err)
				}
			case <-shutdownContext.Done():
				return fmt.Errorf("stop local upload worker: %w", shutdownContext.Err())
			}
		}
		if archiveWorkerStarted {
			select {
			case err := <-archiveDone:
				if err != nil {
					return fmt.Errorf("stop archive worker: %w", err)
				}
			case <-shutdownContext.Done():
				return fmt.Errorf("stop archive worker: %w", shutdownContext.Err())
			}
		}
		if subscriptionWorkerStarted {
			if err := subscriptionService.Wait(shutdownContext); err != nil {
				return fmt.Errorf("stop subscription worker: %w", err)
			}
		}
		if checkinWorkerStarted {
			if err := checkinService.Wait(shutdownContext); err != nil {
				return fmt.Errorf("stop source check-in worker: %w", err)
			}
		}
		if workerStarted {
			if err := workflowService.Wait(shutdownContext); err != nil {
				return fmt.Errorf("stop transfer worker: %w", err)
			}
		}
		return nil
	}
}

func integrationRecords(configurations []config.Integration) []store.IntegrationRecord {
	records := make([]store.IntegrationRecord, 0, len(configurations))
	for _, configuration := range configurations {
		records = append(records, store.IntegrationRecord{
			ID: configuration.ID, Label: configuration.Label, BaseURL: configuration.BaseURL,
		})
	}
	return records
}

func searchSourcesFromSettings(value settings.Values, timeout time.Duration, fixtureMode bool, offline adapter.Offline, sourceProxyURL *url.URL, moviePilotClient *moviepilot.Client) []search.Source {
	if fixtureMode {
		return search.FixtureSources()
	}
	sources := make([]search.Source, 0, len(value.Sources)+1)
	for _, sourceConfiguration := range value.Sources {
		if source := adapter.New(sourceConfiguration, timeout, offline, sourceProxyURL); source != nil {
			sources = append(sources, source)
		}
	}
	if moviePilotClient != nil && moviePilotClient.Configured() {
		sources = append(sources, moviepilot.NewSource(moviePilotClient))
	}
	return sources
}

func parseOptionalHTTPURL(raw string) *url.URL {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return nil
	}
	return parsed
}
