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

type FileDownload interface {
	DownloadFile(ctx context.Context, pickCode, name string) ([]byte, error)
}

type videoFile struct {
	Relative string
	PickCode string
	Size     int64
}

type subtitleFile struct {
	Relative string
	PickCode string
}

type Syncer struct {
	files    Files
	download FileDownload
	mutex    sync.Mutex
	lastList time.Time
}

func New(files Files) *Syncer {
	return &Syncer{files: files}
}

func (s *Syncer) UseDownload(download FileDownload) {
	if s == nil {
		return
	}
	s.mutex.Lock()
	s.download = download
	s.mutex.Unlock()
}

func (s *Syncer) downloader() FileDownload {
	if s == nil {
		return nil
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.download
}

func (s *Syncer) HasVideos(ctx context.Context, folderID string) (bool, error) {
	if s == nil || s.files == nil || strings.TrimSpace(folderID) == "" {
		return false, strm.ErrInvalidRequest
	}
	videos, _, _, err := s.collect(ctx, strm.Request{FileID: folderID})
	if err != nil {
		return false, err
	}
	return len(videos) > 0, nil
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
	videos, subs, folders, err := s.collect(ctx, req)
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
			if req.ContinueOnError {
				result.Failed++
				continue
			}
			result.Duration = time.Since(started)
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
			if req.ContinueOnError {
				result.Failed++
				continue
			}
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
		if !req.DryRun {
			s.writeMatchingSubtitle(ctx, dest, video, subs)
		}
	}
	if req.Prune && !req.DryRun && result.Failed == 0 {
		removed, pruneErr := strm.Prune(req.StrmRootMount, base, keep)
		if pruneErr != nil {
			if req.ContinueOnError {
				result.Failed++
			} else {
				result.Duration = time.Since(started)
				return result, pruneErr
			}
		} else {
			result.Removed = removed
		}
	}
	if result.Created+result.Updated+result.Skipped+result.Removed == 0 && result.Failed == 0 {
		result.Duration = time.Since(started)
		return result, strm.ErrNoVideos
	}
	result.Duration = time.Since(started)
	if result.Failed > 0 {
		return result, strm.ErrPathUnwritable
	}
	return result, nil
}

func (s *Syncer) writeMatchingSubtitle(ctx context.Context, mediaPath string, video videoFile, subs []subtitleFile) {
	download := s.downloader()
	if download == nil || mediaPath == "" {
		return
	}
	sub, ok := matchingSubtitle(video.Relative, subs)
	if !ok {
		return
	}
	var body []byte
	if err := strm.Retry(ctx, func() error {
		var downloadErr error
		body, downloadErr = download.DownloadFile(ctx, sub.PickCode, path.Base(sub.Relative))
		return downloadErr
	}); err != nil || len(body) == 0 {
		return
	}
	_, _ = strm.WriteSidecar(mediaPath, "chi", path.Base(sub.Relative), body)
}

func matchingSubtitle(videoRelative string, subs []subtitleFile) (subtitleFile, bool) {
	stem := subtitleMatchStem(videoRelative)
	var ass subtitleFile
	var other subtitleFile
	hasAss := false
	hasOther := false
	for _, sub := range subs {
		if subtitleMatchStem(sub.Relative) != stem || sub.PickCode == "" {
			continue
		}
		switch strings.ToLower(path.Ext(sub.Relative)) {
		case ".ass", ".ssa":
			ass, hasAss = sub, true
		default:
			other, hasOther = sub, true
		}
	}
	if hasAss {
		return ass, true
	}
	return other, hasOther
}

func subtitleMatchStem(relative string) string {
	stem := strings.TrimSuffix(relative, path.Ext(relative))
	lower := strings.ToLower(stem)
	for _, suffix := range []string{".zh-cn", ".zh-tw", ".zh-hk", ".chi", ".chs", ".cht", ".zh"} {
		if strings.HasSuffix(lower, suffix) {
			return stem[:len(stem)-len(suffix)]
		}
	}
	return stem
}

func (s *Syncer) collect(ctx context.Context, req strm.Request) ([]videoFile, []subtitleFile, map[string]int64, error) {
	if req.IsFile {
		var item drive115.FileItem
		if err := strm.Retry(ctx, func() error {
			var infoErr error
			item, infoErr = s.files.FileInfo(ctx, req.FileID)
			return mapDriveError(infoErr)
		}); err != nil {
			return nil, nil, nil, err
		}
		if _, ok := strm.VideoExtension(item.Name); !ok {
			return nil, nil, nil, strm.ErrNoVideos
		}
		if item.PickCode == "" {
			return nil, nil, nil, strm.ErrListFailed
		}
		return []videoFile{{Relative: item.Name, PickCode: item.PickCode, Size: item.Size}}, nil, map[string]int64{}, nil
	}
	return s.walk(ctx, req.FileID, "", 0, req)
}

func (s *Syncer) walk(ctx context.Context, folderID, prefix string, depth int, req strm.Request) ([]videoFile, []subtitleFile, map[string]int64, error) {
	if depth > maxDepth {
		return nil, nil, nil, strm.ErrListFailed
	}
	folders := map[string]int64{}
	videos := make([]videoFile, 0)
	subs := make([]subtitleFile, 0)
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
			return nil, nil, nil, err
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
				nestedVideos, nestedSubs, nestedFolders, err := s.walk(ctx, item.ID, relative, depth+1, req)
				if err != nil {
					return nil, nil, nil, err
				}
				videos = append(videos, nestedVideos...)
				subs = append(subs, nestedSubs...)
				for id, updated := range nestedFolders {
					folders[id] = updated
				}
				if len(videos) > maxFiles {
					return nil, nil, nil, strm.ErrListFailed
				}
				continue
			}
			_, isVideo := strm.VideoExtension(name)
			_, isSubtitle := strm.SubtitleExtension(name)
			if !isVideo && !isSubtitle {
				continue
			}
			pickCode, err := s.filePickCode(ctx, item)
			if err != nil {
				if isSubtitle {
					continue
				}
				return nil, nil, nil, err
			}
			if isVideo {
				videos = append(videos, videoFile{Relative: relative, PickCode: pickCode, Size: item.Size})
				if len(videos) > maxFiles {
					return nil, nil, nil, strm.ErrListFailed
				}
				continue
			}
			subs = append(subs, subtitleFile{Relative: relative, PickCode: pickCode})
		}
		offset += len(items)
		if len(items) == 0 || offset >= total || len(items) < listPageSize {
			break
		}
	}
	return videos, subs, folders, nil
}

func (s *Syncer) filePickCode(ctx context.Context, item drive115.FileItem) (string, error) {
	pickCode := item.PickCode
	if pickCode != "" {
		return pickCode, nil
	}
	if err := strm.Retry(ctx, func() error {
		info, infoErr := s.files.FileInfo(ctx, item.ID)
		if infoErr != nil {
			return mapDriveError(infoErr)
		}
		pickCode = info.PickCode
		return nil
	}); err != nil {
		return "", err
	}
	if pickCode == "" {
		return "", strm.ErrListFailed
	}
	return pickCode, nil
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
