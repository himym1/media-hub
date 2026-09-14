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
	})
	if !ok || picked.ID != "hash" {
		t.Fatalf("picked=%#v ok=%v", picked, ok)
	}
}

func TestPickChineseSkipsEnglishOnly(t *testing.T) {
	_, ok := PickChinese([]emby.RemoteSubtitle{
		{ID: "en", Name: "English SDH", Language: "eng"},
	})
	if ok {
		t.Fatal("expected no chinese pick")
	}
}
