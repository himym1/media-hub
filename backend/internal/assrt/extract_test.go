package assrt

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestExtractSubtitleReadsAssAndRejectsRar(t *testing.T) {
	name, body, err := extractSubtitle("movie.chi.ass", []byte("[Script Info]\nDialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,你好"), FileHint{})
	if err != nil || name != "movie.chi.ass" || !looksLikeSubtitle(body) {
		t.Fatalf("ass name=%q err=%v", name, err)
	}
	if _, _, err := extractSubtitle("movie.rar", []byte("Rar!\x1a"), FileHint{}); err != ErrUnsupportedFile {
		t.Fatalf("rar err = %v", err)
	}
}

func TestExtractSubtitleOpensZip(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create("folder/movie.chi.srt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\n你好\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	name, body, err := extractSubtitle("bundle.zip", buffer.Bytes(), FileHint{})
	if err != nil || name != "movie.chi.srt" || !looksLikeSubtitle(body) {
		t.Fatalf("zip name=%q err=%v body=%q", name, err, body)
	}
}

func TestExtractZipPrefersMatchingEpisode(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, item := range []struct {
		name string
		body string
	}{
		{"Signal.E01.srt", "1\n00:00:01,000 --> 00:00:02,000\nE01\n"},
		{"Signal.E03.srt", "1\n00:00:01,000 --> 00:00:02,000\nE03\n"},
	} {
		entry, err := writer.Create(item.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(item.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	name, body, err := extractSubtitle("pack.zip", buffer.Bytes(), FileHint{Season: 1, Episode: 3})
	if err != nil || name != "Signal.E03.srt" || !bytes.Contains(body, []byte("E03")) {
		t.Fatalf("name=%q body=%q err=%v", name, body, err)
	}
}

func TestExtractZipPrefersMatchingRelease(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, item := range []struct {
		name string
		body string
	}{
		{"Signal.S01E01.BluRay.srt", "1\n00:00:01,000 --> 00:00:02,000\nBD\n"},
		{"Signal.S01E01.WEB-DL.srt", "1\n00:00:01,000 --> 00:00:02,000\nWEB\n"},
	} {
		entry, err := writer.Create(item.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(item.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	name, body, err := extractSubtitle("pack.zip", buffer.Bytes(), FileHint{
		Season: 1, Episode: 1, FileName: "Signal.S01E01.WEB-DL.1080p.strm",
	})
	if err != nil || name != "Signal.S01E01.WEB-DL.srt" || !bytes.Contains(body, []byte("WEB")) {
		t.Fatalf("name=%q body=%q err=%v", name, body, err)
	}
}
