package adapter

import "testing"

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
}
