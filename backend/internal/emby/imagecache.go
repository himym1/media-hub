package emby

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	primaryImageCacheTTL      = 7 * 24 * time.Hour
	primaryImageCacheMaxFiles = 500
	primaryImageMemoryEntries = 64
)

type primaryImageMemoryEntry struct {
	image   PrimaryImage
	expires time.Time
}

// PrimaryImageCache stores bounded Emby primary posters on disk so clients do not
// refetch them from Emby on every browse.
type PrimaryImageCache struct {
	dir     string
	now     func() time.Time
	mutex   sync.Mutex
	memory  map[string]primaryImageMemoryEntry
	memKeys []string
}

func OpenPrimaryImageCache(dir string) (*PrimaryImageCache, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("poster cache directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create poster cache directory: %w", err)
	}
	return &PrimaryImageCache{
		dir:    dir,
		now:    time.Now,
		memory: make(map[string]primaryImageMemoryEntry, primaryImageMemoryEntries),
	}, nil
}

func (c *PrimaryImageCache) Get(itemID string, maxWidth int) (PrimaryImage, bool) {
	if c == nil || !validEmbyIdentifier(itemID) {
		return PrimaryImage{}, false
	}
	key := cacheKey(itemID, maxWidth)
	if image, ok := c.getMemory(key); ok {
		return image, true
	}
	path := c.filePath(itemID, maxWidth)
	info, err := os.Stat(path)
	if err != nil {
		return PrimaryImage{}, false
	}
	if c.now().Sub(info.ModTime()) > primaryImageCacheTTL {
		_ = os.Remove(path)
		return PrimaryImage{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return PrimaryImage{}, false
	}
	contentType := contentTypeForPath(path)
	if contentType == "" {
		return PrimaryImage{}, false
	}
	image := PrimaryImage{Data: data, ContentType: contentType}
	c.putMemory(key, image)
	return image, true
}

func (c *PrimaryImageCache) Put(itemID string, maxWidth int, image PrimaryImage) error {
	if c == nil || !validEmbyIdentifier(itemID) || len(image.Data) == 0 {
		return nil
	}
	contentType := strings.TrimSpace(strings.Split(image.ContentType, ";")[0])
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" {
		return nil
	}
	key := cacheKey(itemID, maxWidth)
	c.putMemory(key, PrimaryImage{Data: append([]byte(nil), image.Data...), ContentType: contentType})

	c.mutex.Lock()
	defer c.mutex.Unlock()

	target := c.filePath(itemID, maxWidth)
	temp := target + ".tmp"
	if err := os.WriteFile(temp, image.Data, 0o644); err != nil {
		return fmt.Errorf("write poster cache temp file: %w", err)
	}
	if err := os.Rename(temp, target); err != nil {
		_ = os.Remove(temp)
		if writeErr := os.WriteFile(target, image.Data, 0o644); writeErr != nil {
			return fmt.Errorf("write poster cache file: %w", writeErr)
		}
	}
	_ = os.Chtimes(target, c.now(), c.now())
	c.trimLocked()
	return nil
}

func (c *PrimaryImageCache) getMemory(key string) (PrimaryImage, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	entry, ok := c.memory[key]
	if !ok || c.now().After(entry.expires) {
		delete(c.memory, key)
		return PrimaryImage{}, false
	}
	return entry.image, true
}

func (c *PrimaryImageCache) putMemory(key string, image PrimaryImage) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if _, exists := c.memory[key]; !exists {
		c.memKeys = append(c.memKeys, key)
	}
	c.memory[key] = primaryImageMemoryEntry{
		image:   image,
		expires: c.now().Add(primaryImageCacheTTL),
	}
	for len(c.memKeys) > primaryImageMemoryEntries {
		oldest := c.memKeys[0]
		c.memKeys = c.memKeys[1:]
		delete(c.memory, oldest)
	}
}

func (c *PrimaryImageCache) trimLocked() {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	type fileEntry struct {
		name    string
		modTime time.Time
	}
	files := make([]fileEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), cacheFileSuffix()) {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		if c.now().Sub(info.ModTime()) > primaryImageCacheTTL {
			_ = os.Remove(filepath.Join(c.dir, entry.Name()))
			continue
		}
		files = append(files, fileEntry{name: entry.Name(), modTime: info.ModTime()})
	}
	if len(files) <= primaryImageCacheMaxFiles {
		return
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})
	for _, file := range files[:len(files)-primaryImageCacheMaxFiles] {
		_ = os.Remove(filepath.Join(c.dir, file.name))
	}
}

func cacheKey(itemID string, maxWidth int) string {
	return itemID + ":" + fmt.Sprintf("%d", maxWidth)
}

func (c *PrimaryImageCache) filePath(itemID string, maxWidth int) string {
	return filepath.Join(c.dir, fmt.Sprintf("%s_%d%s", itemID, maxWidth, cacheFileSuffix()))
}

func cacheFileSuffix() string {
	return ".poster"
}

func contentTypeForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".poster":
		return "image/jpeg"
	default:
		return ""
	}
}
