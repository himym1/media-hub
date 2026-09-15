package builtin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"media-hub/backend/internal/drive115"
	"media-hub/backend/internal/strm"
)

type filesStub struct {
	userID string
	byID   map[string][]drive115.FileItem
	info   map[string]drive115.FileItem
	err    error
}

func (s filesStub) ListFiles(_ context.Context, parentID string, _, _ int) ([]drive115.FileItem, int, error) {
	if s.err != nil {
		return nil, 0, s.err
	}
	items := s.byID[parentID]
	return items, len(items), nil
}

func (s filesStub) FileInfo(_ context.Context, fileID string) (drive115.FileItem, error) {
	if s.err != nil {
		return drive115.FileItem{}, s.err
	}
	item, ok := s.info[fileID]
	if !ok {
		return drive115.FileItem{}, drive115.ErrUpstreamResponse
	}
	return item, nil
}

func (s filesStub) SessionUserID(context.Context) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.userID, nil
}

func TestSyncWritesMovieFolderSTRM(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "电影")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	syncer := New(filesStub{
		userID: "1001",
		byID: map[string][]drive115.FileItem{
			"folder-1": {
				{ID: "file-1", Name: "Movie.mkv", Kind: "file", PickCode: "pick-1"},
				{ID: "nfo-1", Name: "Movie.nfo", Kind: "file"},
			},
		},
	})
	result, err := syncer.Sync(context.Background(), strm.Request{
		FileID: "folder-1", SourcePath: "电影/Movie (2018)", TargetPath: target,
		StrmBaseURL: "https://strm.example", StrmRootMount: root,
	})
	if err != nil || result.Created != 1 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	body, err := os.ReadFile(filepath.Join(target, "Movie (2018)", "Movie.strm"))
	if err != nil {
		t.Fatal(err)
	}
	want := "https://strm.example/115/url/video.mkv?pickcode=pick-1&userid=1001"
	if string(body) != want {
		t.Fatalf("body=%q", body)
	}
}

func TestSyncWritesReleaseNameWithDotDot(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "电影")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	syncer := New(filesStub{
		userID: "1001",
		byID: map[string][]drive115.FileItem{
			"folder-1": {{ID: "file-1", Name: "Searching.for.Emily.2026.1080p.WEBRip..mkv", Kind: "file", PickCode: "pick-1"}},
		},
	})
	result, err := syncer.Sync(context.Background(), strm.Request{
		FileID: "folder-1", SourcePath: "电影/寻找艾米丽 (2026)", TargetPath: target,
		StrmBaseURL: "https://media.example", StrmRootMount: root,
	})
	if err != nil || result.Created != 1 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	path := filepath.Join(target, "寻找艾米丽 (2026)", "Searching.for.Emily.2026.1080p.WEBRip.strm")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestSyncMapsUnauthorizedToAuthExpired(t *testing.T) {
	syncer := New(filesStub{err: drive115.ErrUnauthorized})
	_, err := syncer.Sync(context.Background(), strm.Request{
		FileID: "folder-1", TargetPath: "/media/电影", StrmBaseURL: "https://qms.example", StrmRootMount: "/media",
	})
	if err != strm.ErrAuthExpired {
		t.Fatalf("err=%v", err)
	}
}

func TestSyncLibraryRootDoesNotDuplicateFolderName(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "电影")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	syncer := New(filesStub{
		userID: "1001",
		byID: map[string][]drive115.FileItem{
			"dest": {{ID: "file-1", Name: "Movie.mkv", Kind: "file", PickCode: "pick-1"}},
		},
	})
	if _, err := syncer.Sync(context.Background(), strm.Request{
		FileID: "dest", TargetPath: target, LibraryRoot: true,
		StrmBaseURL: "https://media.example", StrmRootMount: root,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "Movie.strm")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "电影", "Movie.strm")); !os.IsNotExist(err) {
		t.Fatal("library root should not append the destination folder name")
	}
}

func TestSyncSkipsSmallVideosAndPrunesStaleSTRM(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "电影")
	stale := filepath.Join(target, "Gone.strm")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	syncer := New(filesStub{
		userID: "1",
		byID: map[string][]drive115.FileItem{
			"dest": {
				{ID: "small", Name: "Small.mkv", Kind: "file", PickCode: "pick-small", Size: 1024},
				{ID: "ok", Name: "Keep.mkv", Kind: "file", PickCode: "pick-ok", Size: strm.DefaultMinVideoSize + 1},
			},
		},
	})
	result, err := syncer.Sync(context.Background(), strm.Request{
		FileID: "dest", TargetPath: target, LibraryRoot: true, Prune: true,
		MinVideoSize: strm.DefaultMinVideoSize, StrmBaseURL: "https://media.example", StrmRootMount: root,
	})
	if err != nil || result.Created != 1 || result.Skipped != 1 || result.Removed != 1 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("stale strm should be pruned")
	}
}

func TestSyncIncrementalSkipsUnchangedFolders(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "电影")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	syncer := New(filesStub{
		userID: "1",
		byID: map[string][]drive115.FileItem{
			"dest":  {{ID: "child", Name: "Movie (2018)", Kind: "folder", UpdatedAt: 99}},
			"child": {{ID: "file-1", Name: "Movie.mkv", Kind: "file", PickCode: "pick-1"}},
		},
	})
	result, err := syncer.Sync(context.Background(), strm.Request{
		FileID: "dest", TargetPath: target, LibraryRoot: true, Incremental: true,
		KnownFolders: map[string]int64{"child": 99}, StrmBaseURL: "https://media.example", StrmRootMount: root,
	})
	if !errors.Is(err, strm.ErrNoVideos) || result.Scanned != 0 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestSyncContinuesOtherFilesWhenOneWriteFails(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "电影")
	blockedDir := filepath.Join(target, "Blocked (2018)")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blockedDir, []byte("not-a-directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	syncer := New(filesStub{
		userID: "1",
		byID: map[string][]drive115.FileItem{
			"dest": {
				{ID: "blocked-dir", Name: "Blocked (2018)", Kind: "folder"},
				{ID: "keep-dir", Name: "Keep (2018)", Kind: "folder"},
			},
			"blocked-dir": {{ID: "blocked", Name: "Blocked.mkv", Kind: "file", PickCode: "pick-blocked"}},
			"keep-dir":    {{ID: "ok", Name: "Keep.mkv", Kind: "file", PickCode: "pick-ok"}},
		},
	})
	result, err := syncer.Sync(context.Background(), strm.Request{
		FileID: "dest", TargetPath: target, LibraryRoot: true, ContinueOnError: true, Prune: true,
		StrmBaseURL: "https://media.example", StrmRootMount: root,
	})
	if !errors.Is(err, strm.ErrPathUnwritable) || result.Failed == 0 || result.Created != 1 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if _, statErr := os.Stat(filepath.Join(target, "Keep (2018)", "Keep.strm")); statErr != nil {
		t.Fatal(statErr)
	}
	if info, statErr := os.Stat(blockedDir); statErr != nil || info.IsDir() {
		t.Fatal("failed write should not replace the blocking path")
	}
}

type downloadStub struct {
	byPick map[string][]byte
}

func (s downloadStub) DownloadFile(_ context.Context, pickCode, _ string) ([]byte, error) {
	body, ok := s.byPick[pickCode]
	if !ok {
		return nil, errors.New("missing subtitle")
	}
	return body, nil
}

func TestSyncWritesMatchingChineseSidecar(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "电视剧")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	syncer := New(filesStub{
		userID: "1001",
		byID: map[string][]drive115.FileItem{
			"folder-1": {
				{ID: "file-1", Name: "Signal 2016 E03.mkv", Kind: "file", PickCode: "pick-video"},
				{ID: "sub-1", Name: "Signal 2016 E03.ass", Kind: "file", PickCode: "pick-sub"},
				{ID: "other-1", Name: "readme.txt", Kind: "file"},
			},
		},
	})
	syncer.UseDownload(downloadStub{byPick: map[string][]byte{"pick-sub": []byte("[Script Info]\nDialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,你好")}})
	result, err := syncer.Sync(context.Background(), strm.Request{
		FileID: "folder-1", SourcePath: "电视剧/信号 (2016)", TargetPath: target,
		StrmBaseURL: "https://media.example", StrmRootMount: root,
	})
	if err != nil || result.Created != 1 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	sidecar := filepath.Join(target, "信号 (2016)", "Signal 2016 E03.chi.ass")
	body, err := os.ReadFile(sidecar)
	if err != nil || !strings.Contains(string(body), "你好") {
		t.Fatalf("sidecar=%q err=%v", body, err)
	}
}

func TestMatchingSubtitleAcceptsLanguageTaggedAss(t *testing.T) {
	sub, ok := matchingSubtitle("Movie.mkv", []subtitleFile{{
		Relative: "Movie.chs.ass", PickCode: "pick-sub",
	}})
	if !ok || sub.Relative != "Movie.chs.ass" {
		t.Fatalf("got %#v ok=%v", sub, ok)
	}
}

func TestHasVideosReportsEmptyAndReadyFolders(t *testing.T) {
	syncer := New(filesStub{
		userID: "1",
		byID: map[string][]drive115.FileItem{
			"empty": {},
			"ready": {{ID: "video-1", Name: "Movie.mkv", Kind: "file", PickCode: "pick-1", Size: 1024}},
		},
	})
	ready, err := syncer.HasVideos(context.Background(), "empty")
	if err != nil || ready {
		t.Fatalf("empty folder ready=%v err=%v", ready, err)
	}
	ready, err = syncer.HasVideos(context.Background(), "ready")
	if err != nil || !ready {
		t.Fatalf("ready folder ready=%v err=%v", ready, err)
	}
}

func TestSyncDoesNotClearSessionOnListFailure(t *testing.T) {
	syncer := New(filesStub{userID: "1", err: drive115.ErrUpstreamResponse})
	_, err := syncer.Sync(context.Background(), strm.Request{
		FileID: "folder-1", TargetPath: "/media/电影", StrmBaseURL: "https://qms.example", StrmRootMount: "/media",
	})
	if err != strm.ErrListFailed {
		t.Fatalf("err=%v", err)
	}
}
