package strm

import (
	"context"
	"testing"
	"time"

	"media-hub/backend/internal/config"
	"media-hub/backend/internal/integration"
)

type coordinatorSyncer struct {
	calls int
	req   Request
	err   error
}

func (s *coordinatorSyncer) Sync(_ context.Context, req Request) (Result, error) {
	s.calls++
	s.req = req
	if s.err != nil {
		return Result{}, s.err
	}
	return Result{Scanned: 1, Created: 1, Folders: map[string]int64{"folder": 10}}, nil
}

type sessionStub struct{ err error }

func (s sessionStub) SessionUserID(context.Context) (string, error) {
	return "1001", s.err
}

func TestCoordinatorHealthReportsUnwritableMount(t *testing.T) {
	coordinator := NewCoordinator(nil, &coordinatorSyncer{}, sessionStub{}, nil, func() config.Workflow {
		return config.Workflow{}
	}, nil)
	health := coordinator.Check(context.Background())
	if health.ID != "strm" || health.Status != integration.StatusDegraded || health.Detail != "STRM 挂载不可写" {
		t.Fatalf("health=%#v", health)
	}
}

func TestCoordinatorLibrarySyncUsesLibraryRoot(t *testing.T) {
	syncer := &coordinatorSyncer{}
	coordinator := NewCoordinator(nil, syncer, sessionStub{}, nil, func() config.Workflow {
		return config.Workflow{
			SyncMode:      config.SyncModeBuiltin,
			StrmBaseURL:   "https://media.example",
			StrmRootMount: "/media",
			Movie:         config.WorkflowTarget{DestinationID: "100", QMediaSyncTargetPath: "/media/电影", EmbyLibraryID: "15075"},
		}
	}, nil)
	if _, err := coordinator.SyncLibrary(context.Background(), LibrarySyncInput{MediaType: "movie", Full: true}); err != nil {
		t.Fatal(err)
	}
	if !syncer.req.LibraryRoot || !syncer.req.Prune || !syncer.req.ContinueOnError || syncer.req.FileID != "100" || syncer.req.MinVideoSize != DefaultMinVideoSize {
		t.Fatalf("request=%#v", syncer.req)
	}
}

func TestCoordinatorRecordsAuthErrorWithoutClearingSession(t *testing.T) {
	syncer := &coordinatorSyncer{err: ErrAuthExpired}
	coordinator := NewCoordinator(nil, syncer, sessionStub{}, nil, func() config.Workflow {
		return config.Workflow{SyncMode: config.SyncModeBuiltin, StrmBaseURL: "https://media.example", StrmRootMount: "/tmp"}
	}, nil)
	_, err := coordinator.SyncTransfer(context.Background(), Request{FileID: "1"})
	if err != ErrAuthExpired {
		t.Fatalf("err=%v", err)
	}
	status, _ := coordinator.Status(context.Background())
	if status.LastError != "115 授权已失效" {
		t.Fatalf("status=%#v", status)
	}
}

func TestCoordinatorRejectsOverlappingLibrarySync(t *testing.T) {
	syncer := &coordinatorSyncer{}
	coordinator := NewCoordinator(nil, syncer, sessionStub{}, nil, func() config.Workflow {
		return config.Workflow{
			SyncMode:      config.SyncModeBuiltin,
			StrmBaseURL:   "https://media.example",
			StrmRootMount: "/media",
			Movie:         config.WorkflowTarget{DestinationID: "100", QMediaSyncTargetPath: "/media/电影", EmbyLibraryID: "15075"},
			Series:        config.WorkflowTarget{DestinationID: "200", QMediaSyncTargetPath: "/media/电视剧", EmbyLibraryID: "15077"},
		}
	}, nil)
	coordinator.mutex.Lock()
	coordinator.running = true
	coordinator.mutex.Unlock()
	if err := coordinator.EnqueueLibrarySync(LibrarySyncInput{MediaType: "all"}); err != ErrBusy {
		t.Fatalf("err=%v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if syncer.calls != 0 {
		t.Fatalf("calls=%d", syncer.calls)
	}
}
