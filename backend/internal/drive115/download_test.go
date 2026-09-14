package drive115

import (
	"context"
	"strings"
	"testing"
)

func TestDownloadHTTPSRejectsNonHTTPS(t *testing.T) {
	_, err := downloadHTTPS(context.Background(), "http://example.invalid/sub.ass", "ua")
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("err=%v", err)
	}
}
