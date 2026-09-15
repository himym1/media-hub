package strm

import "testing"

func TestURLMatchesExistingQMSFormat(t *testing.T) {
	got := URL("https://strm.example/", ".mkv", "pick-example", "1001")
	want := "https://strm.example/115/url/video.mkv?pickcode=pick-example&userid=1001"
	if got != want {
		t.Fatalf("url=%q want=%q", got, want)
	}
}

func TestVideoExtensionAcceptsKnownContainers(t *testing.T) {
	if ext, ok := VideoExtension("Movie.MKV"); !ok || ext != ".mkv" {
		t.Fatalf("ext=%q ok=%v", ext, ok)
	}
	if _, ok := VideoExtension("notes.txt"); ok {
		t.Fatal("text files must not be treated as video")
	}
}

func TestSubtitleExtensionAcceptsTextSubs(t *testing.T) {
	if ext, ok := SubtitleExtension("Show.E01.ass"); !ok || ext != ".ass" {
		t.Fatalf("ext=%q ok=%v", ext, ok)
	}
	if _, ok := SubtitleExtension("Show.E01.mkv"); ok {
		t.Fatal("videos must not be treated as subtitles")
	}
}
