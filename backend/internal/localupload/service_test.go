package localupload

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListKeepsPathsInsideConfiguredRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "movie.mkv"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	service := NewService(nil, nil, []string{root})
	if roots := service.Roots(); len(roots) != 1 || roots[0].ID != "root-1" {
		t.Fatalf("roots=%v", roots)
	}
	items, err := service.List("root-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "movie.mkv" {
		t.Fatalf("items=%v", items)
	}
	if _, err := service.List("root-1", "escape"); err == nil {
		t.Fatal("expected symlink escape rejection")
	}
	if _, err := service.List("root-1", "../"); err == nil {
		t.Fatal("expected parent traversal rejection")
	}
}
