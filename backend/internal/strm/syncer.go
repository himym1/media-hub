package strm

import (
	"context"
	"fmt"
	"time"
)

const (
	ModeQMediaSync = "qmediasync"
	ModeBuiltin    = "builtin"

	DefaultMinVideoSize = 100 * 1024 * 1024
	DefaultSyncEvery    = 30 * time.Minute
	FullSyncEvery       = 6 * time.Hour
)

type Request struct {
	FileID        string
	SourcePath    string
	TargetPath    string
	IsFile        bool
	LibraryRoot   bool
	Prune         bool
	Incremental   bool
	DryRun        bool
	MinVideoSize  int64
	KnownFolders  map[string]int64
	StrmBaseURL   string
	StrmRootMount string
}

type Result struct {
	Scanned  int
	Created  int
	Updated  int
	Skipped  int
	Removed  int
	Folders  map[string]int64
	Duration time.Duration
}

func (r Result) Summary() string {
	return fmt.Sprintf("扫描 %d，新建 %d，更新 %d，跳过 %d，删除 %d，用时 %s", r.Scanned, r.Created, r.Updated, r.Skipped, r.Removed, r.Duration.Round(time.Millisecond))
}

type Syncer interface {
	Sync(context.Context, Request) (Result, error)
}
