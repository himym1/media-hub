package strm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLibraryFileMapsEmbySTRMPath(t *testing.T) {
	mount := t.TempDir()
	resolved, err := ResolveLibraryFile(mount, "/media3/115-strm/电视剧/信号/S01E01.strm")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(mount, "电视剧", "信号", "S01E01.strm")
	if resolved != want {
		t.Fatalf("resolved = %q want %q", resolved, want)
	}
}

func TestResolveLibraryFileKeepsSameMount(t *testing.T) {
	mount := t.TempDir()
	path := filepath.Join(mount, "电影", "movie.strm")
	resolved, err := ResolveLibraryFile(mount, path)
	if err != nil || resolved != path {
		t.Fatalf("resolved = %q err=%v", resolved, err)
	}
}

func TestWriteSidecarNextToSTRM(t *testing.T) {
	dir := t.TempDir()
	media := filepath.Join(dir, "S01E01.strm")
	if err := os.WriteFile(media, []byte("https://example/115/url/x"), 0o664); err != nil {
		t.Fatal(err)
	}
	dest, err := WriteSidecar(media, "chi", "signal.chi.ass", []byte("[Script Info]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if dest != filepath.Join(dir, "S01E01.chi.ass") {
		t.Fatalf("dest = %q", dest)
	}
	body, err := os.ReadFile(dest)
	if err != nil || string(body) != "[Script Info]\n" {
		t.Fatalf("sidecar = %q err=%v", body, err)
	}
}
