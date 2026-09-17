package adapter

import (
	"errors"
	"strings"
	"testing"
)

func TestParse115ShareURL(t *testing.T) {
	code, receive, err := Parse115ShareURL("https://115.com/s/shareABC123?password=WENG&from=tg", "")
	if err != nil || code != "shareABC123" || receive != "WENG" {
		t.Fatalf("code=%q receive=%q err=%v", code, receive, err)
	}
	code, receive, err = Parse115ShareURL("anxia.com/s/shareABC123", "xy9z")
	if err != nil || code != "shareABC123" || receive != "xy9z" {
		t.Fatalf("fallback code=%q receive=%q err=%v", code, receive, err)
	}
	if _, _, err := Parse115ShareURL("https://pan.quark.cn/s/nope", "abcd"); err == nil {
		t.Fatal("quark URL should be rejected")
	}
}

func TestParseAdultImport(t *testing.T) {
	share, err := ParseAdultImport("https://115.com/s/shareABC123?password=ab12", "")
	if err != nil || share.Kind != ImportKindShare || share.ShareCode != "shareABC123" || share.ReceiveCode != "ab12" {
		t.Fatalf("share=%#v err=%v", share, err)
	}
	direct, err := ParseAdultImport("https://cdn.example.com/clip/SSIS-001.mkv?token=1", "")
	if err != nil || direct.Kind != ImportKindURL || !strings.Contains(direct.URL, "SSIS-001.mkv") {
		t.Fatalf("direct=%#v err=%v", direct, err)
	}
	magnet, err := ParseAdultImport("magnet:?xt=urn:btih:"+strings.Repeat("a", 40), "")
	if err != nil || magnet.Kind != ImportKindURL {
		t.Fatalf("magnet=%#v err=%v", magnet, err)
	}
	if _, err := ParseAdultImport("https://pan.quark.cn/s/nope", "abcd"); !errors.Is(err, ErrOtherCloudImport) {
		t.Fatalf("quark err=%v", err)
	}
	if _, err := ParseAdultImport("https://115.com/?cid=1", ""); !errors.Is(err, ErrNeed115ShareOrFile) {
		t.Fatalf("115 page err=%v", err)
	}
}

func TestAdultLibraryTitlePrefersCode(t *testing.T) {
	if got := adultLibraryTitle("随便看看", "SSIS-001.1080p.mkv", "extra.mp4"); got != "SSIS-001" {
		t.Fatalf("got %q", got)
	}
	if got := adultLibraryTitle("", "fc2-ppv-1234567.mp4"); got != "FC2-PPV-1234567" {
		t.Fatalf("fc2=%q", got)
	}
	if got := adultLibraryTitle("自制标题", "clip.mp4"); got != "自制标题" {
		t.Fatalf("title=%q", got)
	}
	if ExtractAdultCode("SSIS-001.1080p") != "SSIS-001" || ExtractAdultCode("127.0.0.1") != "" {
		t.Fatal("extract adult code")
	}
}
