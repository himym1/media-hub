package emby

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPrimaryImageCacheStoresAndReusesPosters(t *testing.T) {
	dir := t.TempDir()
	now := time.Unix(1_700_000_000, 0)
	cache, err := OpenPrimaryImageCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	cache.now = func() time.Time { return now }

	image := PrimaryImage{Data: []byte{0xff, 0xd8, 0xff, 0xd9}, ContentType: "image/jpeg"}
	if err := cache.Put("item-1", 320, image); err != nil {
		t.Fatal(err)
	}
	got, ok := cache.Get("item-1", 320)
	if !ok || string(got.Data) != string(image.Data) || got.ContentType != image.ContentType {
		t.Fatalf("cache hit = %#v ok=%v", got, ok)
	}

	cache.now = func() time.Time { return now.Add(primaryImageCacheTTL + time.Minute) }
	if _, ok := cache.Get("item-1", 320); ok {
		t.Fatal("expected expired cache entry to miss")
	}
	if _, err := os.Stat(filepath.Join(dir, "item-1_320.poster")); !os.IsNotExist(err) {
		t.Fatalf("expected expired cache file removed, err=%v", err)
	}
}

func TestPrimaryImageCacheTrimsOldestFiles(t *testing.T) {
	dir := t.TempDir()
	now := time.Unix(1_700_000_000, 0)
	cache, err := OpenPrimaryImageCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	cache.now = func() time.Time { return now }

	image := PrimaryImage{Data: []byte{0xff, 0xd8, 0xff, 0xd9}, ContentType: "image/jpeg"}
	for index := 0; index < primaryImageCacheMaxFiles+3; index++ {
		itemID := fmt.Sprintf("item-%04d", index)
		if err := cache.Put(itemID, 320, image); err != nil {
			t.Fatal(err)
		}
		now = now.Add(time.Second)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) > primaryImageCacheMaxFiles {
		t.Fatalf("cache files = %d want <= %d", len(entries), primaryImageCacheMaxFiles)
	}
}
