package strm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ResolveLibraryFile(mount, embyPath string) (string, error) {
	mount = filepath.Clean(strings.TrimSpace(mount))
	embyPath = filepath.Clean(filepath.FromSlash(strings.ReplaceAll(strings.TrimSpace(embyPath), "\\", "/")))
	if mount == "" || !filepath.IsAbs(mount) || embyPath == "" {
		return "", fmt.Errorf("library file path is invalid")
	}
	if embyPath == mount || strings.HasPrefix(embyPath, mount+string(os.PathSeparator)) {
		return embyPath, nil
	}
	parts := strings.Split(filepath.ToSlash(embyPath), "/")
	for index, part := range parts {
		if part == "115-strm" && index+1 < len(parts) {
			rel := filepath.FromSlash(strings.Join(parts[index+1:], "/"))
			return filepath.Join(mount, rel), nil
		}
	}
	if len(parts) >= 2 {
		start := len(parts) - 3
		if start < 0 {
			start = 0
		}
		candidate := filepath.Join(mount, filepath.FromSlash(strings.Join(parts[start:], "/")))
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("library file is outside the STRM mount")
}

func WriteSidecar(mediaPath, language, sourceName string, body []byte) (string, error) {
	if len(body) == 0 {
		return "", fmt.Errorf("subtitle body is empty")
	}
	ext := sidecarExtension(sourceName)
	stem := strings.TrimSuffix(mediaPath, filepath.Ext(mediaPath))
	if language = sanitizeLanguage(language); language == "" {
		language = "chi"
	}
	dest := stem + "." + language + ext
	if !strings.HasPrefix(dest, filepath.Dir(mediaPath)+string(os.PathSeparator)) {
		return "", fmt.Errorf("subtitle path is invalid")
	}
	temp := dest + ".tmp"
	if err := os.WriteFile(temp, body, 0o664); err != nil {
		return "", err
	}
	if err := os.Rename(temp, dest); err != nil {
		_ = os.Remove(temp)
		return "", err
	}
	return dest, nil
}

func sidecarExtension(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".ass"):
		return ".ass"
	case strings.HasSuffix(lower, ".ssa"):
		return ".ssa"
	default:
		return ".srt"
	}
}

func sanitizeLanguage(value string) string {
	var builder strings.Builder
	for _, character := range strings.ToLower(strings.TrimSpace(value)) {
		if character >= 'a' && character <= 'z' {
			builder.WriteRune(character)
		}
	}
	if builder.Len() < 2 || builder.Len() > 8 {
		return ""
	}
	return builder.String()
}
