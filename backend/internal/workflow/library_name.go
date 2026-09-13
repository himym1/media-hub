package workflow

import (
	"fmt"
	"path"
	"strings"
	"unicode"
)

// libraryEntryName builds a stable Emby/STRM-friendly name from the job title.
func libraryEntryName(title string, year int, isFile bool, currentPath string) string {
	name := sanitizeLibraryName(title)
	if name == "" {
		return ""
	}
	if year > 0 {
		name = fmt.Sprintf("%s (%d)", name, year)
	}
	if isFile {
		if ext := path.Ext(strings.TrimSpace(currentPath)); ext != "" && len(ext) <= 12 {
			name += ext
		}
	}
	return name
}

func sanitizeLibraryName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(value))
	lastWasSpace := false
	for _, r := range value {
		switch {
		case r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|':
			continue
		case unicode.IsControl(r):
			continue
		case unicode.IsSpace(r):
			if lastWasSpace || builder.Len() == 0 {
				continue
			}
			builder.WriteByte(' ')
			lastWasSpace = true
		default:
			builder.WriteRune(r)
			lastWasSpace = false
		}
	}
	name := strings.TrimSpace(builder.String())
	for strings.HasSuffix(name, ".") {
		name = strings.TrimSpace(strings.TrimSuffix(name, "."))
	}
	if len([]rune(name)) > 120 {
		name = string([]rune(name)[:120])
		name = strings.TrimSpace(name)
	}
	return name
}

func pathBaseName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimRight(value, "/")
	if value == "" {
		return ""
	}
	return path.Base(value)
}

func needsLibraryRename(currentPath, desired string) bool {
	desired = strings.TrimSpace(desired)
	if desired == "" {
		return false
	}
	current := pathBaseName(currentPath)
	if current == "" {
		return true
	}
	return !strings.EqualFold(current, desired)
}

func shouldRenameTransferredFolder(fileID, destinationID, currentPath, desired string) bool {
	fileID = strings.TrimSpace(fileID)
	destinationID = strings.TrimSpace(destinationID)
	if fileID == "" || fileID == destinationID {
		return false
	}
	return needsLibraryRename(currentPath, desired)
}
