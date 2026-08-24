package strm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPruneRemovesMissingVideosAndEmptyDirs(t *testing.T) {
	root := t.TempDir()
	keepPath := filepath.Join(root, "Keep", "Keep.strm")
	dropPath := filepath.Join(root, "Drop", "Drop.strm")
	if err := os.MkdirAll(filepath.Dir(keepPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dropPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keepPath, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dropPath, []byte("drop"), 0o644); err != nil {
		t.Fatal(err)
	}
	removed, err := Prune(root, root, map[string]struct{}{"Keep/Keep.strm": {}})
	if err != nil || removed != 1 {
		t.Fatalf("removed=%d err=%v", removed, err)
	}
	if _, err := os.Stat(keepPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dropPath); !os.IsNotExist(err) {
		t.Fatal("stale strm should be removed")
	}
	if _, err := os.Stat(filepath.Join(root, "Drop")); !os.IsNotExist(err) {
		t.Fatal("empty directory should be removed")
	}
}
