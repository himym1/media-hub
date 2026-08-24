package strm

import (
	"path"
	"path/filepath"
	"strings"
)

func LocalBase(targetPath, sourcePath string, isFile bool) string {
	target := filepath.Clean(strings.TrimSpace(targetPath))
	if isFile {
		return target
	}
	entry := path.Base(strings.TrimRight(strings.ReplaceAll(sourcePath, "\\", "/"), "/"))
	if entry == "" || entry == "." || entry == "/" {
		return target
	}
	return filepath.Join(target, entry)
}

func LocalSTRMPath(rootMount, localBase, relativeVideo string) (string, error) {
	root := filepath.Clean(strings.TrimSpace(rootMount))
	base := filepath.Clean(strings.TrimSpace(localBase))
	if !filepath.IsAbs(root) || !filepath.IsAbs(base) || !within(root, base) {
		return "", ErrPathUnwritable
	}
	raw := strings.TrimSpace(strings.ReplaceAll(relativeVideo, "\\", "/"))
	if raw == "" || strings.Contains(raw, "..") {
		return "", ErrPathUnwritable
	}
	relative := strings.TrimPrefix(path.Clean("/"+raw), "/")
	if relative == "" || relative == "." {
		return "", ErrPathUnwritable
	}
	ext, ok := VideoExtension(relative)
	if !ok {
		return "", ErrInvalidRequest
	}
	dest := filepath.Clean(filepath.Join(base, strings.TrimSuffix(relative, ext)+".strm"))
	if !within(root, dest) || !within(base, dest) {
		return "", ErrPathUnwritable
	}
	return dest, nil
}

func within(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
