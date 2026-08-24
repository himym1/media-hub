package strm

import "testing"

func TestURLMatchesExistingQMSFormat(t *testing.T) {
	got := URL("https://qms.himym.us.ci/", ".mkv", "bib9gv7oqzrog88cc", "103539243")
	want := "https://qms.himym.us.ci/115/url/video.mkv?pickcode=bib9gv7oqzrog88cc&userid=103539243"
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
