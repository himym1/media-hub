package captureupload

import (
	"context"
	"errors"
	"testing"

	"media-hub/backend/internal/config"
	"media-hub/backend/internal/drive115"
	"media-hub/backend/internal/workflow"
)

type driveStub struct {
	parent     string
	name       string
	folderID   string
	uploadDest string
	files      []drive115.FileItem
}

func (s *driveStub) EnsureFolder(_ context.Context, parentID, name string) (string, error) {
	s.parent, s.name = parentID, name
	return s.folderID, nil
}

func (s *driveStub) InitSampleUpload(_ context.Context, destinationID, filename string, size int64) (drive115.SampleUpload, error) {
	s.uploadDest = destinationID
	return drive115.SampleUpload{
		Host: "https://bucket.oss-cn-shenzhen.aliyuncs.com", Object: "obj", AccessID: "id",
		Policy: "p", Signature: "s", Callback: "cb", Target: "U_1_" + destinationID, Filename: filename,
	}, nil
}

func (s *driveStub) ListFiles(context.Context, string, int, int) ([]drive115.FileItem, int, error) {
	return s.files, len(s.files), nil
}

type transferStub struct {
	title    string
	folderID string
}

func (transferStub) Workflow() config.Workflow {
	return config.Workflow{
		StrmBaseURL:   "https://media.example",
		StrmRootMount: "/media",
		Adult:         config.WorkflowTarget{DestinationID: "300", QMediaSyncTargetPath: "/strm/adult", EmbyLibraryID: "adult"},
	}
}

func (s *transferStub) EnqueueUploadedImport(_ context.Context, _ int64, title, folderID, _ string) (workflow.Job, bool, error) {
	s.title, s.folderID = title, folderID
	return workflow.Job{ID: "job-1", Title: title, MediaType: "adult", Source: "share", State: "queued"}, true, nil
}

func TestInitCreatesAdultFolderAndTicket(t *testing.T) {
	drive := &driveStub{folderID: "9001"}
	service := New(drive, &transferStub{})
	ticket, err := service.Init(context.Background(), "SSIS-001.ts", 8, "")
	if err != nil {
		t.Fatal(err)
	}
	if drive.parent != "300" || drive.name != "SSIS-001" || drive.uploadDest != "9001" {
		t.Fatalf("drive=%#v", drive)
	}
	if ticket.DestinationID != "9001" || ticket.Title != "SSIS-001" || ticket.Filename != "SSIS-001.ts" || ticket.Host == "" {
		t.Fatalf("ticket=%#v", ticket)
	}
}

func TestCompleteRequiresUploadedFile(t *testing.T) {
	drive := &driveStub{files: []drive115.FileItem{{Kind: "file", Name: "other.ts"}}}
	transfers := &transferStub{}
	service := New(drive, transfers)
	if _, _, err := service.Complete(context.Background(), 1, "9001", "clip.ts", "clip", "key_one_1"); !errors.Is(err, ErrNotReady) {
		t.Fatalf("err=%v", err)
	}
	drive.files = []drive115.FileItem{{Kind: "file", Name: "clip.ts"}}
	job, created, err := service.Complete(context.Background(), 1, "9001", "clip.ts", "clip", "key_one_1")
	if err != nil || !created || job.Title != "clip" || transfers.folderID != "9001" {
		t.Fatalf("job=%#v created=%v err=%v dest=%q", job, created, err, transfers.folderID)
	}
}
