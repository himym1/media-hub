package captureupload

import (
	"context"
	"errors"
	"path"
	"strings"

	"media-hub/backend/internal/adapter"
	"media-hub/backend/internal/config"
	"media-hub/backend/internal/drive115"
	"media-hub/backend/internal/workflow"
)

var (
	ErrUnavailable = errors.New("capture upload unavailable")
	ErrInvalid     = errors.New("invalid capture upload")
	ErrNotReady    = errors.New("capture file not in 115 yet")
)

const maxUploadBytes = 5 * 1024 * 1024 * 1024

type Drive interface {
	EnsureFolder(context.Context, string, string) (string, error)
	InitSampleUpload(context.Context, string, string, int64) (drive115.SampleUpload, error)
	ListFiles(context.Context, string, int, int) ([]drive115.FileItem, int, error)
}

type Transfers interface {
	Workflow() config.Workflow
	EnqueueUploadedImport(context.Context, int64, string, string, string) (workflow.Job, bool, error)
}

type Ticket struct {
	DestinationID string `json:"destinationId"`
	Filename      string `json:"filename"`
	Title         string `json:"title"`
	Target        string `json:"target"`
	Host          string `json:"host"`
	Object        string `json:"object"`
	AccessID      string `json:"accessid"`
	Policy        string `json:"policy"`
	Signature     string `json:"signature"`
	Callback      string `json:"callback"`
}

type Service struct {
	drive     Drive
	transfers Transfers
}

func New(drive Drive, transfers Transfers) *Service {
	return &Service{drive: drive, transfers: transfers}
}

func (s *Service) Init(ctx context.Context, filename string, size int64, title string) (Ticket, error) {
	if s == nil || s.drive == nil || s.transfers == nil {
		return Ticket{}, ErrUnavailable
	}
	target, ok := s.transfers.Workflow().Target("adult")
	if !ok {
		return Ticket{}, workflow.ErrTargetUnavailable
	}
	filename = path.Base(strings.ReplaceAll(strings.TrimSpace(filename), "\\", "/"))
	if filename == "" || filename == "." || size < 1 || size > maxUploadBytes {
		return Ticket{}, ErrInvalid
	}
	folderTitle := adapter.AdultLibraryTitle(title, strings.TrimSuffix(filename, path.Ext(filename)))
	folderID, err := s.drive.EnsureFolder(ctx, target.DestinationID, folderTitle)
	if err != nil {
		return Ticket{}, err
	}
	upload, err := s.drive.InitSampleUpload(ctx, folderID, filename, size)
	if err != nil {
		return Ticket{}, err
	}
	return Ticket{
		DestinationID: folderID,
		Filename:      upload.Filename,
		Title:         folderTitle,
		Target:        upload.Target,
		Host:          upload.Host,
		Object:        upload.Object,
		AccessID:      upload.AccessID,
		Policy:        upload.Policy,
		Signature:     upload.Signature,
		Callback:      upload.Callback,
	}, nil
}

func (s *Service) Complete(ctx context.Context, userID int64, destinationID, filename, title, key string) (workflow.Job, bool, error) {
	if s == nil || s.drive == nil || s.transfers == nil {
		return workflow.Job{}, false, ErrUnavailable
	}
	destinationID = strings.TrimSpace(destinationID)
	filename = path.Base(strings.ReplaceAll(strings.TrimSpace(filename), "\\", "/"))
	title = adapter.AdultLibraryTitle(title, strings.TrimSuffix(filename, path.Ext(filename)))
	if destinationID == "" || destinationID == "0" || filename == "" {
		return workflow.Job{}, false, ErrInvalid
	}
	items, _, err := s.drive.ListFiles(ctx, destinationID, 100, 0)
	if err != nil {
		return workflow.Job{}, false, err
	}
	found := false
	for _, item := range items {
		if item.Kind == "file" && item.Name == filename {
			found = true
			break
		}
	}
	if !found {
		return workflow.Job{}, false, ErrNotReady
	}
	return s.transfers.EnqueueUploadedImport(ctx, userID, title, destinationID, key)
}
