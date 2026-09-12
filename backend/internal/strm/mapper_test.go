package strm

import (
	"path/filepath"
	"testing"
)

func TestLocalBaseIncludesFolderName(t *testing.T) {
	got := LocalBase("/media/电影", "电影/头号玩家 (2018)", false)
	want := filepath.Join("/media/电影", "头号玩家 (2018)")
	if got != want {
		t.Fatalf("base=%q want=%q", got, want)
	}
	if LocalBase("/media/电影", "电影/single.mkv", true) != filepath.Clean("/media/电影") {
		t.Fatal("file transfers should write under the target path")
	}
}

func TestLocalSTRMPathStaysInsideMount(t *testing.T) {
	dest, err := LocalSTRMPath("/media", "/media/电影/Movie (2018)", "Video.mkv")
	if err != nil {
		t.Fatal(err)
	}
	if dest != filepath.Join("/media/电影/Movie (2018)", "Video.strm") {
		t.Fatalf("dest=%q", dest)
	}
	if _, err := LocalSTRMPath("/media", "/tmp/outside", "Video.mkv"); err != ErrPathUnwritable {
		t.Fatalf("outside target error=%v", err)
	}
	if _, err := LocalSTRMPath("/media", "/media/电影", "../escape.mkv"); err != ErrPathUnwritable {
		t.Fatalf("escape error=%v", err)
	}
}

func TestLocalSTRMPathAllowsDotDotInReleaseName(t *testing.T) {
	dest, err := LocalSTRMPath("/media", "/media/电影/寻找艾米丽 (2026)", "Searching.for.Emily.2026.1080p.WEBRip..mkv")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/media/电影/寻找艾米丽 (2026)", "Searching.for.Emily.2026.1080p.WEBRip.strm")
	if dest != want {
		t.Fatalf("dest=%q want=%q", dest, want)
	}
	if _, err := LocalSTRMPath("/media", "/media/电影", "folder/../escape.mkv"); err != ErrPathUnwritable {
		t.Fatalf("segment escape error=%v", err)
	}
}
