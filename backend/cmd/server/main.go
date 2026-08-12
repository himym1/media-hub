package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"media-hub/backend/internal/androidrelease"
	"media-hub/backend/internal/archive"
	"media-hub/backend/internal/auth"
	"media-hub/backend/internal/config"
	"media-hub/backend/internal/drive115"
	"media-hub/backend/internal/emby"
	"media-hub/backend/internal/httpapi"
	"media-hub/backend/internal/integration"
	"media-hub/backend/internal/localupload"
	"media-hub/backend/internal/qms"
	"media-hub/backend/internal/search"
	"media-hub/backend/internal/securepayload"
	"media-hub/backend/internal/selection"
	"media-hub/backend/internal/statistics"
	"media-hub/backend/internal/store"
	"media-hub/backend/internal/subscription"
	"media-hub/backend/internal/subx"
	"media-hub/backend/internal/subxmigration"
	"media-hub/backend/internal/tmdb"
	"media-hub/backend/internal/webui"
	"media-hub/backend/internal/wecom"
	"media-hub/backend/internal/workflow"
)

var version = "0.6.0-dev"

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
	qmsClient := qms.NewClient(
		configuration.QMediaSync.BaseURL,
		configuration.QMediaSync.APIKey,
		configuration.ProbeTimeout,
	)
	embyClient := emby.NewClient(
		configuration.Emby.BaseURL,
		configuration.Emby.APIKey,
		configuration.ProbeTimeout,
		configuration.Emby.UserID,
	)
	drive115Client := drive115.NewClient(
		configuration.Drive115.AccessToken,
		configuration.ProbeTimeout,
	)
	drive115AuthService := drive115.NewAuthService(
		dataStore, securePayloadCodec, configuration.Drive115.ClientID, drive115Client, configuration.ProbeTimeout,
	)
	drive115CommandService := drive115.NewCommandService(dataStore, securePayloadCodec)
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
	wecomClient := wecom.NewClient(
		configuration.WeCom.BaseURL,
		configuration.WeCom.CorpID,
		configuration.WeCom.Secret,
		configuration.WeCom.ChatID,
		configuration.ProbeTimeout,
	)
	subxClient := subx.NewClient(configuration.SubX, configuration.ProbeTimeout)
	subxService := subx.NewService(dataStore, subxClient, securePayloadCodec)
	var identityResolver search.IdentityResolver
	if configuration.TMDB.BaseURL != "" && configuration.TMDB.AccessToken != "" {
		identityResolver = tmdbClient
	}
	sources := searchSources(configuration)
	if subxClient.Configured() && configuration.SubX.SourceEnabled {
		sources = append(sources, subx.NewSource(subxClient, subxService))
	}
	searchService := search.NewServiceWithIdentity(identityResolver, sources...)
	workflowService := workflow.NewService(
		dataStore, searchService, selectionCodec, qmsClient, embyClient, wecomClient, configuration.Workflow,
	)
	subscriptionService := subscription.NewService(dataStore, searchService, workflowService, embyClient)
	overview := integration.NewOverviewService(
		subxClient,
		searchService,
		drive115AuthService,
		tmdbClient,
		wecomClient,
		qmsClient,
		embyClient,
	)
	_, movieTargetReady := configuration.Workflow.Target("movie")
	_, seriesTargetReady := configuration.Workflow.Target("series")
	migrationService := subxmigration.NewService(
		subxService, subscriptionService, configuration.SubX.SourceEnabled,
		subxmigration.ReadinessRequirements{
			CoreConfigurationReady: qmsClient.Configured() && embyClient.Configured() && configuration.TMDB.BaseURL != "" && configuration.TMDB.AccessToken != "" && drive115AuthService.Configured() && movieTargetReady && seriesTargetReady,
			NativeSourceCount:      len(configuration.Sources), ParallelValidationCompleted: configuration.SubXParallelValidated, Health: overview,
		},
	)
	statisticsService := statistics.NewService(dataStore)
	localUploadService := localupload.NewService(dataStore, securePayloadCodec, configuration.LocalUploadRoots)
	archiveService := archive.NewService(dataStore, securePayloadCodec, drive115AuthService)
	androidReleaseService := androidrelease.NewService(configuration.AndroidReleaseDir)

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
	subxWorkerStarted := false
	if configuration.SubX.SourceEnabled {
		if err := subxService.Start(runtimeContext); err != nil {
			return fmt.Errorf("start SubX migration-source worker: %w", err)
		}
		subxWorkerStarted = true
	}
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
			QMediaSync: qmsClient, Emby: embyClient, Drive115: drive115AuthService, Drive115Auth: drive115AuthService, Drive115Commands: drive115CommandService,
			Workflow: workflowService, Subscriptions: subscriptionService, Migration: migrationService, Statistics: statisticsService, LocalUploads: localUploadService, Archive: archiveService, AndroidReleases: androidReleaseService,
			SecureCookies: configuration.SecureCookies,
			Web:           webui.Handler(),
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
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
		if subxWorkerStarted {
			waitContext, cancelWait := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancelWait()
			if waitErr := subxService.Wait(waitContext); waitErr != nil {
				return fmt.Errorf("stop SubX compatibility worker: %w", waitErr)
			}
		}
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
		if subxWorkerStarted {
			if err := subxService.Wait(shutdownContext); err != nil {
				return fmt.Errorf("stop SubX compatibility worker: %w", err)
			}
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

func searchSources(configuration config.Config) []search.Source {
	if configuration.FixtureMode {
		return search.FixtureSources()
	}

	sources := make([]search.Source, 0, len(configuration.Sources))
	for _, sourceConfiguration := range configuration.Sources {
		if sourceConfiguration.BaseURL == "" {
			continue
		}
		sources = append(sources, search.NewHTTPSource(
			sourceConfiguration.ID,
			sourceConfiguration.Label,
			sourceConfiguration.BaseURL,
			sourceConfiguration.Token,
			configuration.ProbeTimeout,
		))
	}
	return sources
}
