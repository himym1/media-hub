package strm

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	if _, err := os.Stat(filepath.Join(dir, "S01E01.chi.default.ass")); err != nil {
		t.Fatalf("default sidecar: %v", err)
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
	if err := os.WriteFile(filepath.Join(dir, "S01E01.chi.default.ass"), []byte("[Script Info]\n"), 0o664); err != nil {
		t.Fatal(err)
	}
	if err := RemoveSidecars(media); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
		t.Fatalf("sidecar still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "S01E01.chi.default.ass")); !os.IsNotExist(err) {
		t.Fatalf("default sidecar still exists: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("neighbor sidecar was removed: %v", err)
	}
}

func TestReadSidecarAcceptsZhCN(t *testing.T) {
	dir := t.TempDir()
	media := filepath.Join(dir, "Movie.strm")
	if err := os.WriteFile(media, []byte("https://example/115/url/x"), 0o664); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Movie.zh-CN.srt"), []byte("1\n00:00:01,000 --> 00:00:02,000\n你好\n"), 0o664); err != nil {
		t.Fatal(err)
	}
	sidecar, err := ReadSidecar(media, "chi")
	if err != nil || sidecar.Name != "chi.srt" || !strings.Contains(string(sidecar.Body), "你好") {
		t.Fatalf("sidecar=%#v err=%v", sidecar, err)
	}
}

func TestPromoteExternalSidecarCopiesZhCN(t *testing.T) {
	dir := t.TempDir()
	media := filepath.Join(dir, "Movie.strm")
	if err := os.WriteFile(media, []byte("https://example/115/url/x"), 0o664); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Movie.zh-CN.srt"), []byte("1\n00:00:01,000 --> 00:00:02,000\n你好\n"), 0o664); err != nil {
		t.Fatal(err)
	}
	if err := PromoteExternalSidecar(media); err != nil {
		t.Fatal(err)
	}
	sidecar, err := ReadSidecar(media, "chi")
	if err != nil || !strings.Contains(string(sidecar.Body), "你好") {
		t.Fatalf("sidecar=%#v err=%v", sidecar, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Movie.chi.srt")); err != nil {
		t.Fatalf("chi sidecar: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Movie.chi.default.srt")); err != nil {
		t.Fatalf("default sidecar: %v", err)
	}
}

func TestResolveMappedFilePrefersLongestPrefix(t *testing.T) {
	maps, err := ParsePathMap("/media2:/local-media2,/media/links:/local-links,/media:/wrong")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveMappedFile(maps, "/media/links/电影/Local.mkv")
	if err != nil || resolved != "/local-links/电影/Local.mkv" {
		t.Fatalf("resolved=%q err=%v", resolved, err)
	}
	resolved, err = ResolveMappedFile(maps, "/media2/av/file.mp4")
	if err != nil || resolved != "/local-media2/av/file.mp4" {
		t.Fatalf("adult resolved=%q err=%v", resolved, err)
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
