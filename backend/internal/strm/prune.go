package strm

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Prune(rootMount, localBase string, keep map[string]struct{}) (int, error) {
	root := filepath.Clean(rootMount)
	base := filepath.Clean(localBase)
	if !filepath.IsAbs(root) || !filepath.IsAbs(base) || !within(root, base) {
		return 0, ErrPathUnwritable
	}
	info, err := os.Stat(base)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, ErrPathUnwritable
	}
	if !info.IsDir() {
		return 0, ErrPathUnwritable
	}
	removed := 0
	err = filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".strm") {
			return nil
		}
		rel, relErr := filepath.Rel(base, path)
		if relErr != nil || !within(base, path) {
			return ErrPathUnwritable
		}
		key := filepath.ToSlash(rel)
		if _, ok := keep[key]; ok {
			return nil
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return ErrPathUnwritable
		}
		removed++
		return nil
	})
	if err != nil {
		return removed, err
	}
	return removed, removeEmptyDirs(base)
}

func removeEmptyDirs(root string) error {
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() || path == root {
			return err
		}
		dirs = append(dirs, path)
		return nil
	})
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			continue
		}
		_ = os.Remove(dir)
	}
	return nil
}
