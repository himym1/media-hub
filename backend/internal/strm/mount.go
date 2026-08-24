package strm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func MountWritable(path string) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%w: mount is not absolute", ErrPathUnwritable)
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("%w: mount is missing", ErrPathUnwritable)
	}
	probe := filepath.Join(path, ".media-hub-strm-write-check")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		return fmt.Errorf("%w: mount is not writable", ErrPathUnwritable)
	}
	_ = os.Remove(probe)
	return nil
}
