package subtitles

import (
	"testing"

	"media-hub/backend/internal/emby"
)

func TestPickChinesePrefersSimplifiedHashMatch(t *testing.T) {
	picked, ok := PickChinese([]emby.RemoteSubtitle{
		{ID: "en", Name: "English", Language: "eng", DownloadCount: 9000},
		{ID: "web", Name: "Movie.2018.WEB.Chs", Format: "srt", DownloadCount: 100},
		{ID: "hash", Name: "Movie.chi.ass", Format: "ass", IsHashMatch: true},
	}, "")
	if !ok || picked.ID != "hash" {
		t.Fatalf("picked=%#v ok=%v", picked, ok)
	}
}

func TestPickChineseSkipsEnglishOnly(t *testing.T) {
	_, ok := PickChinese([]emby.RemoteSubtitle{
		{ID: "en", Name: "English SDH", Language: "eng"},
	}, "")
	if ok {
		t.Fatal("expected no chinese pick")
	}
}

func TestPickChinesePrefersBlurayOverDvdRip(t *testing.T) {
	picked, ok := PickChinese([]emby.RemoteSubtitle{
		{ID: "dvd", Name: "Dagon.2001.DVDRip.chs", Format: "srt", DownloadCount: 80},
		{ID: "bd", Name: "Dagon.2001.1080p.BluRay.Chs", Format: "srt", DownloadCount: 12},
	}, "Dagon.2001.1080p.BluRay.x264-ENCOUNTERS.strm")
	if !ok || picked.ID != "bd" {
		t.Fatalf("picked=%#v ok=%v", picked, ok)
	}
}
