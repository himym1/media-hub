package assrt

import "testing"

func TestParseReleaseReadsSourceResolutionAndGroup(t *testing.T) {
	profile := ParseRelease("Signal.S01E01.WEB-DL.1080p.HThoreau.strm")
	if _, ok := profile.Sources["webdl"]; !ok {
		t.Fatalf("sources = %#v", profile.Sources)
	}
	if _, ok := profile.Resolutions["1080p"]; !ok {
		t.Fatalf("resolutions = %#v", profile.Resolutions)
	}
	if _, ok := profile.Groups["hthoreau"]; !ok {
		t.Fatalf("groups = %#v", profile.Groups)
	}
}

func TestParseReleaseReadsBracketGroup(t *testing.T) {
	profile := ParseRelease("[Nekomoe kissaten] Signal S01E01 (BDRip 1080p)")
	if _, ok := profile.Groups["nekomoe kissaten"]; !ok {
		t.Fatalf("groups = %#v", profile.Groups)
	}
	if _, ok := profile.Sources["bluray"]; !ok {
		t.Fatalf("sources = %#v", profile.Sources)
	}
}

func TestScoreHitPrefersMatchingRelease(t *testing.T) {
	target := SearchTarget{FileName: "Signal.S01E01.WEB-DL.1080p-Nekomoe.strm"}
	web := ScoreHit(Hit{Name: "信号 Signal", VideoName: "Signal.S01E01.WEB-DL.1080p"}, target)
	bluray := ScoreHit(Hit{Name: "信号 Signal", VideoName: "Signal.S01E01.BluRay.1080p"}, target)
	if web <= bluray {
		t.Fatalf("web=%d bluray=%d", web, bluray)
	}
}

func TestScoreHitPenalizesDvdRipOnBluray(t *testing.T) {
	target := SearchTarget{FileName: "Dagon.2001.1080p.BluRay.x264-ENCOUNTERS.strm"}
	bluray := ScoreHit(Hit{Name: "Dagon.2001.1080p.BluRay.Chs"}, target)
	dvd := ScoreHit(Hit{Name: "Dagon.2001.DVDRip.chs"}, target)
	if bluray <= dvd {
		t.Fatalf("bluray=%d dvd=%d", bluray, dvd)
	}
}

func TestScoreHitRewardsReleaseGroup(t *testing.T) {
	target := SearchTarget{FileName: "[LoliHouse] Signal S01E01 WEB-DL 1080p.strm"}
	matched := ScoreHit(Hit{Name: "[LoliHouse] 信号 WEB-DL"}, target)
	other := ScoreHit(Hit{Name: "[Nekomoe] 信号 WEB-DL"}, target)
	if matched <= other {
		t.Fatalf("matched=%d other=%d", matched, other)
	}
}
