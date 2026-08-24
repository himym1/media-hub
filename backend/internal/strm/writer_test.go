package strm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileCreatesUpdatesAndSkips(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Movie (2018)", "Movie.strm")
	created, updated, err := WriteFile(path, "first")
	if err != nil || !created || updated {
		t.Fatalf("create created=%v updated=%v err=%v", created, updated, err)
	}
	created, updated, err = WriteFile(path, "first")
	if err != nil || created || updated {
		t.Fatalf("skip created=%v updated=%v err=%v", created, updated, err)
	}
	created, updated, err = WriteFile(path, "second")
	if err != nil || created || !updated {
		t.Fatalf("update created=%v updated=%v err=%v", created, updated, err)
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != "second" {
		t.Fatalf("body=%q err=%v", body, err)
	}
}
