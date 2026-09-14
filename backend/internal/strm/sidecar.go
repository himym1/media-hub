package strm

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrSidecarNotFound = errors.New("local subtitle sidecar not found")

const maxSidecarBytes = 8 << 20

type Sidecar struct {
	Name        string
	ContentType string
	Body        []byte
}

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
	if err := writeDefaultSidecar(dest, body); err != nil {
		return dest, err
	}
	return dest, nil
}

func ReadSidecar(mediaPath, language string) (Sidecar, error) {
	mediaPath = filepath.Clean(mediaPath)
	if mediaPath == "" || !filepath.IsAbs(mediaPath) {
		return Sidecar{}, fmt.Errorf("library file path is invalid")
	}
	dir := filepath.Dir(mediaPath)
	stem := strings.TrimSuffix(mediaPath, filepath.Ext(mediaPath))
	languages := sidecarLanguages(language)
	for _, lang := range languages {
		for _, ext := range []string{".ass", ".ssa", ".srt"} {
			dest := stem + "." + lang + ext
			if !strings.HasPrefix(dest, dir+string(os.PathSeparator)) {
				continue
			}
			body, err := os.ReadFile(dest)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return Sidecar{}, err
			}
			if len(body) == 0 || len(body) > maxSidecarBytes {
				continue
			}
			return Sidecar{
				Name:        lang + ext,
				ContentType: sidecarContentType(ext),
				Body:        body,
			}, nil
		}
	}
	return Sidecar{}, ErrSidecarNotFound
}

func RemoveSidecars(mediaPath string) error {
	mediaPath = filepath.Clean(mediaPath)
	if mediaPath == "" || !filepath.IsAbs(mediaPath) {
		return fmt.Errorf("library file path is invalid")
	}
	dir := filepath.Dir(mediaPath)
	stem := strings.TrimSuffix(mediaPath, filepath.Ext(mediaPath))
	for _, lang := range []string{"chi", "zh", "chs"} {
		for _, ext := range []string{".ass", ".ssa", ".srt"} {
			dest := stem + "." + lang + ext
			if !strings.HasPrefix(dest, dir+string(os.PathSeparator)) {
				continue
			}
			if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
				return err
			}
			if err := os.Remove(defaultSidecarPath(dest)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func PromoteExternalSidecar(mediaPath string) error {
	if sidecar, err := ReadSidecar(mediaPath, "chi"); err == nil {
		dest := strings.TrimSuffix(filepath.Clean(mediaPath), filepath.Ext(mediaPath)) + "." + sidecar.Name
		return writeDefaultSidecar(dest, sidecar.Body)
	}
	mediaPath = filepath.Clean(mediaPath)
	dir := filepath.Dir(mediaPath)
	stem := strings.TrimSuffix(mediaPath, filepath.Ext(mediaPath))
	base := filepath.Base(stem)
	for _, extra := range []string{".zh-CN.srt", ".zh-CN.ass", ".zh-cn.srt", ".zh-cn.ass", ".zh.srt", ".zh.ass", ".chs.srt", ".chs.ass"} {
		source := filepath.Join(dir, base+extra)
		if !strings.HasPrefix(source, dir+string(os.PathSeparator)) {
			continue
		}
		body, err := os.ReadFile(source)
		if err != nil || len(body) == 0 || len(body) > maxSidecarBytes {
			continue
		}
		if _, err := WriteSidecar(mediaPath, "chi", base+extra, body); err != nil {
			return err
		}
		return nil
	}
	return ErrSidecarNotFound
}

func writeDefaultSidecar(dest string, body []byte) error {
	defaultDest := defaultSidecarPath(dest)
	if defaultDest == dest {
		return nil
	}
	dir := filepath.Dir(dest)
	if !strings.HasPrefix(defaultDest, dir+string(os.PathSeparator)) {
		return fmt.Errorf("subtitle path is invalid")
	}
	temp := defaultDest + ".tmp"
	if err := os.WriteFile(temp, body, 0o664); err != nil {
		return err
	}
	if err := os.Rename(temp, defaultDest); err != nil {
		_ = os.Remove(temp)
		return err
	}
	return nil
}

func defaultSidecarPath(dest string) string {
	ext := filepath.Ext(dest)
	if ext == "" {
		return dest
	}
	return strings.TrimSuffix(dest, ext) + ".default" + ext
}

func sidecarLanguages(language string) []string {
	lang := sanitizeLanguage(language)
	if lang == "" {
		lang = "chi"
	}
	if lang == "chi" {
		return []string{"chi", "zh", "chs"}
	}
	return []string{lang}
}

func sidecarContentType(ext string) string {
	switch ext {
	case ".ass", ".ssa":
		return "text/x-ssa"
	default:
		return "application/x-subrip"
	}
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
