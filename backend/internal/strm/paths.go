package strm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PathMapping struct {
	EmbyPrefix string
	LocalRoot  string
}

func ParsePathMap(raw string) ([]PathMapping, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	mappings := make([]PathMapping, 0)
	seen := make(map[string]struct{})
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		idx := strings.Index(item, ":/")
		if idx <= 0 {
			return nil, fmt.Errorf("library path map entry %q must look like /emby-prefix:/local-root", item)
		}
		embyPrefix := filepath.Clean(filepath.FromSlash(item[:idx]))
		localRoot := filepath.Clean(filepath.FromSlash(item[idx+1:]))
		if !filepath.IsAbs(embyPrefix) || !filepath.IsAbs(localRoot) {
			return nil, fmt.Errorf("library path map entry %q must use absolute paths", item)
		}
		if _, exists := seen[embyPrefix]; exists {
			continue
		}
		seen[embyPrefix] = struct{}{}
		mappings = append(mappings, PathMapping{EmbyPrefix: embyPrefix, LocalRoot: localRoot})
	}
	return mappings, nil
}

func ResolveMappedFile(maps []PathMapping, embyPath string) (string, error) {
	embyPath = filepath.Clean(filepath.FromSlash(strings.ReplaceAll(strings.TrimSpace(embyPath), "\\", "/")))
	if embyPath == "" || !filepath.IsAbs(embyPath) {
		return "", fmt.Errorf("library file path is invalid")
	}
	best := PathMapping{}
	bestLen := -1
	for _, mapping := range maps {
		prefix := filepath.Clean(mapping.EmbyPrefix)
		if embyPath == prefix || strings.HasPrefix(embyPath, prefix+string(os.PathSeparator)) {
			if len(prefix) > bestLen {
				best = mapping
				bestLen = len(prefix)
			}
		}
	}
	if bestLen < 0 {
		return "", fmt.Errorf("library file is outside mapped mounts")
	}
	rel, err := filepath.Rel(filepath.Clean(best.EmbyPrefix), embyPath)
	if err != nil {
		return "", fmt.Errorf("library file path is invalid")
	}
	resolved := filepath.Join(filepath.Clean(best.LocalRoot), rel)
	if !strings.HasPrefix(resolved, filepath.Clean(best.LocalRoot)+string(os.PathSeparator)) && resolved != filepath.Clean(best.LocalRoot) {
		return "", fmt.Errorf("library file path is invalid")
	}
	return resolved, nil
}

func ResolveMediaFile(strmMount string, maps []PathMapping, embyPath string) (string, error) {
	if mapped, err := ResolveMappedFile(maps, embyPath); err == nil {
		return mapped, nil
	}
	return ResolveLibraryFile(strmMount, embyPath)
}
