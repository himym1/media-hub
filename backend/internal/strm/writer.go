package strm

import (
	"os"
	"path/filepath"
	"strings"
)

func WriteFile(path, content string) (created, updated bool, err error) {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) || strings.HasSuffix(path, string(filepath.Separator)) {
		return false, false, ErrPathUnwritable
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, false, ErrPathUnwritable
	}
	current, readErr := os.ReadFile(path)
	if readErr == nil && string(current) == content {
		return false, false, nil
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return false, false, ErrPathUnwritable
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return false, false, ErrPathUnwritable
	}
	if readErr == nil {
		return false, true, nil
	}
	return true, false, nil
}
