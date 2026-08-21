package workflow

import "testing"

func TestLibraryEntryNameSanitizesReleaseWatermarks(t *testing.T) {
	name := libraryEntryName("头号玩家", 2018, false, "/Media/【站点】头号玩家.Ready.Player.One")
	if name != "头号玩家 (2018)" {
		t.Fatalf("name=%q", name)
	}
	fileName := libraryEntryName("Movie: Part/1", 2024, true, "/Media/Movie.mkv")
	if fileName != "Movie Part1 (2024).mkv" {
		t.Fatalf("file name=%q", fileName)
	}
}

func TestNeedsLibraryRename(t *testing.T) {
	if !needsLibraryRename("/Media/【站点】头号玩家", "头号玩家 (2018)") {
		t.Fatal("expected rename for watermark folder")
	}
	if needsLibraryRename("/Media/头号玩家 (2018)", "头号玩家 (2018)") {
		t.Fatal("expected clean folder to skip rename")
	}
}
