package strm

import (
	"errors"
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
	sidecar, err := ReadSidecar(media, "chi")
	if err != nil || sidecar.Name != "chi.ass" || sidecar.ContentType != "text/x-ssa" || string(sidecar.Body) != "[Script Info]\n" {
		t.Fatalf("sidecar=%#v err=%v", sidecar, err)
	}
}

func TestRemoveSidecarsDeletesLanguageFiles(t *testing.T) {
	dir := t.TempDir()
	media := filepath.Join(dir, "S01E01.strm")
	if err := os.WriteFile(media, []byte("https://example/115/url/x"), 0o664); err != nil {
		t.Fatal(err)
	}
	sidecar := filepath.Join(dir, "S01E01.chi.ass")
	other := filepath.Join(dir, "S01E02.chi.ass")
	if err := os.WriteFile(sidecar, []byte("[Script Info]\n"), 0o664); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("[Script Info]\n"), 0o664); err != nil {
		t.Fatal(err)
	}
	if err := RemoveSidecars(media); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
		t.Fatalf("sidecar still exists: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("neighbor sidecar was removed: %v", err)
	}
}

func TestReadSidecarMissing(t *testing.T) {
	dir := t.TempDir()
	media := filepath.Join(dir, "S01E01.strm")
	if err := os.WriteFile(media, []byte("https://example/115/url/x"), 0o664); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSidecar(media, "chi"); !errors.Is(err, ErrSidecarNotFound) {
		t.Fatalf("err=%v", err)
	}
}
