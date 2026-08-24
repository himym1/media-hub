package builtin

import (
	"context"
	"errors"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"media-hub/backend/internal/drive115"
	"media-hub/backend/internal/strm"
)

const (
	listPageSize = 115
	maxFiles     = 5000
	maxDepth     = 16
	listGap      = 400 * time.Millisecond
)

type Files interface {
	ListFiles(ctx context.Context, parentID string, limit, offset int) ([]drive115.FileItem, int, error)
	FileInfo(ctx context.Context, fileID string) (drive115.FileItem, error)
	SessionUserID(ctx context.Context) (string, error)
}

type videoFile struct {
	Relative string
	PickCode string
	Size     int64
}

type Syncer struct {
	files    Files
	mutex    sync.Mutex
	lastList time.Time
}

func New(files Files) *Syncer {
	return &Syncer{files: files}
}

func (s *Syncer) Sync(ctx context.Context, req strm.Request) (strm.Result, error) {
	started := time.Now()
	if s == nil || s.files == nil {
		return strm.Result{}, strm.ErrInvalidRequest
	}
	if strings.TrimSpace(req.FileID) == "" || strings.TrimSpace(req.TargetPath) == "" || strings.TrimSpace(req.StrmBaseURL) == "" || strings.TrimSpace(req.StrmRootMount) == "" {
		return strm.Result{}, strm.ErrInvalidRequest
	}
	var userID string
	if err := strm.Retry(ctx, func() error {
		var err error
		userID, err = s.files.SessionUserID(ctx)
		return mapDriveError(err)
	}); err != nil {
		return strm.Result{}, err
	}
	if err := strm.MountWritable(req.StrmRootMount); err != nil {
		return strm.Result{}, err
	}
	videos, folders, err := s.collect(ctx, req)
	if err != nil {
		return strm.Result{}, err
	}
	result := strm.Result{Scanned: len(videos), Folders: folders}
	if len(videos) == 0 && !req.Prune {
		result.Duration = time.Since(started)
		return result, strm.ErrNoVideos
	}
	base := strm.LocalBase(req.TargetPath, req.SourcePath, req.IsFile || req.LibraryRoot)
	keep := map[string]struct{}{}
	for _, video := range videos {
		ext, ok := strm.VideoExtension(video.Relative)
		if !ok || video.PickCode == "" {
			result.Skipped++
			continue
		}
		if req.MinVideoSize > 0 && video.Size > 0 && video.Size < req.MinVideoSize {
			result.Skipped++
			continue
		}
		dest, err := strm.LocalSTRMPath(req.StrmRootMount, base, video.Relative)
		if err != nil {
			return result, err
		}
		if rel, relErr := filepath.Rel(base, dest); relErr == nil {
			keep[filepath.ToSlash(rel)] = struct{}{}
		}
		if req.DryRun {
			result.Created++
			continue
		}
		content := strm.URL(req.StrmBaseURL, ext, video.PickCode, userID)
		var created, updated bool
		if err := strm.Retry(ctx, func() error {
			var writeErr error
			created, updated, writeErr = strm.WriteFile(dest, content)
			return writeErr
		}); err != nil {
			result.Duration = time.Since(started)
			return result, err
		}
		switch {
		case created:
			result.Created++
		case updated:
			result.Updated++
		default:
			result.Skipped++
		}
	}
	if req.Prune && !req.DryRun {
		removed, pruneErr := strm.Prune(req.StrmRootMount, base, keep)
		if pruneErr != nil {
			result.Duration = time.Since(started)
			return result, pruneErr
		}
		result.Removed = removed
	}
	if result.Created+result.Updated+result.Skipped+result.Removed == 0 {
		result.Duration = time.Since(started)
		return result, strm.ErrNoVideos
	}
	result.Duration = time.Since(started)
	return result, nil
}

func (s *Syncer) collect(ctx context.Context, req strm.Request) ([]videoFile, map[string]int64, error) {
	if req.IsFile {
		var item drive115.FileItem
		if err := strm.Retry(ctx, func() error {
			var infoErr error
			item, infoErr = s.files.FileInfo(ctx, req.FileID)
			return mapDriveError(infoErr)
		}); err != nil {
			return nil, nil, err
		}
		if _, ok := strm.VideoExtension(item.Name); !ok {
			return nil, nil, strm.ErrNoVideos
		}
		if item.PickCode == "" {
			return nil, nil, strm.ErrListFailed
		}
		return []videoFile{{Relative: item.Name, PickCode: item.PickCode, Size: item.Size}}, map[string]int64{}, nil
	}
	return s.walk(ctx, req.FileID, "", 0, req)
}

func (s *Syncer) walk(ctx context.Context, folderID, prefix string, depth int, req strm.Request) ([]videoFile, map[string]int64, error) {
	if depth > maxDepth {
		return nil, nil, strm.ErrListFailed
	}
	folders := map[string]int64{}
	videos := make([]videoFile, 0)
	offset := 0
	for {
		var items []drive115.FileItem
		var total int
		if err := strm.Retry(ctx, func() error {
			if err := s.throttle(ctx); err != nil {
				return err
			}
			var listErr error
			items, total, listErr = s.files.ListFiles(ctx, folderID, listPageSize, offset)
			return mapDriveError(listErr)
		}); err != nil {
			return nil, nil, err
		}
		for _, item := range items {
			name := strings.TrimSpace(item.Name)
			if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\x00") {
				continue
			}
			relative := name
			if prefix != "" {
				relative = path.Join(prefix, name)
			}
			if item.Kind == "folder" {
				folders[item.ID] = item.UpdatedAt
				if req.Incremental && req.KnownFolders != nil {
					if previous, ok := req.KnownFolders[item.ID]; ok && previous > 0 && previous == item.UpdatedAt {
						continue
					}
				}
				nestedVideos, nestedFolders, err := s.walk(ctx, item.ID, relative, depth+1, req)
				if err != nil {
					return nil, nil, err
				}
				videos = append(videos, nestedVideos...)
				for id, updated := range nestedFolders {
					folders[id] = updated
				}
				if len(videos) > maxFiles {
					return nil, nil, strm.ErrListFailed
				}
				continue
			}
			if _, ok := strm.VideoExtension(name); !ok {
				continue
			}
			pickCode := item.PickCode
			if pickCode == "" {
				if err := strm.Retry(ctx, func() error {
					info, infoErr := s.files.FileInfo(ctx, item.ID)
					if infoErr != nil {
						return mapDriveError(infoErr)
					}
					pickCode = info.PickCode
					return nil
				}); err != nil {
					return nil, nil, err
				}
			}
			if pickCode == "" {
				return nil, nil, strm.ErrListFailed
			}
			videos = append(videos, videoFile{Relative: relative, PickCode: pickCode, Size: item.Size})
			if len(videos) > maxFiles {
				return nil, nil, strm.ErrListFailed
			}
		}
		offset += len(items)
		if len(items) == 0 || offset >= total || len(items) < listPageSize {
			break
		}
	}
	return videos, folders, nil
}

func (s *Syncer) throttle(ctx context.Context) error {
	s.mutex.Lock()
	wait := listGap - time.Since(s.lastList)
	s.mutex.Unlock()
	if wait <= 0 {
		s.mutex.Lock()
		s.lastList = time.Now()
		s.mutex.Unlock()
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
	}
	s.mutex.Lock()
	s.lastList = time.Now()
	s.mutex.Unlock()
	return nil
}

func mapDriveError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, drive115.ErrUnauthorized) || errors.Is(err, drive115.ErrNotConfigured) {
		return strm.ErrAuthExpired
	}
	return strm.ErrListFailed
}
