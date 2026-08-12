package archive

import "testing"

func TestSuggestNameRequiresReviewAndPreservesExtension(t *testing.T) {
	name, confidence := suggestName("Example.Movie.2024.2160p.WEB-DL.H265.mkv")
	if name != "Example Movie (2024).mkv" || confidence != "review" {
		t.Fatalf("suggestion = %q, %q", name, confidence)
	}
}

func TestArchiveStepsRejectImplicitOrUnsafeWrites(t *testing.T) {
	cases := []Step{
		{Operation: "rename", FileID: "1", Name: ""},
		{Operation: "rename", FileID: "1", Name: "../private"},
		{Operation: "move", FileID: "1", TargetParentID: "root"},
		{Operation: "delete", FileID: "1"},
	}
	for _, value := range cases {
		if validStep(value) {
			t.Fatalf("accepted unsafe step: %#v", value)
		}
	}
	if !validStep(Step{Operation: "rename", FileID: "10", Name: "Example (2024)"}) ||
		!validStep(Step{Operation: "move", FileID: "10", TargetParentID: "20"}) {
		t.Fatal("rejected valid explicit step")
	}
}
