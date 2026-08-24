package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSTRMSyncStateRoundTrip(t *testing.T) {
	ctx := context.Background()
	dataStore, err := Open(ctx, filepath.Join(t.TempDir(), "media-hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()

	if err := dataStore.ReplaceSTRMFolderStates(ctx, "/media/电影", map[string]int64{"folder-1": 100}, false); err != nil {
		t.Fatal(err)
	}
	if err := dataStore.ReplaceSTRMFolderStates(ctx, "/media/电影", map[string]int64{"folder-2": 200}, true); err != nil {
		t.Fatal(err)
	}
	states, err := dataStore.STRMFolderStates(ctx, "/media/电影")
	if err != nil || states["folder-1"] != 100 || states["folder-2"] != 200 {
		t.Fatalf("states=%v err=%v", states, err)
	}
	if err := dataStore.InsertSTRMSyncRun(ctx, STRMSyncRun{
		MediaType: "movie", Full: true, Created: 2, Finished: 50,
	}); err != nil {
		t.Fatal(err)
	}
	run, ok, err := dataStore.LatestSTRMSyncRun(ctx)
	if err != nil || !ok || !run.Full || run.Created != 2 {
		t.Fatalf("run=%#v ok=%v err=%v", run, ok, err)
	}
	fullAt, err := dataStore.LatestFullSTRMSyncAt(ctx)
	if err != nil || fullAt != 50 {
		t.Fatalf("fullAt=%d err=%v", fullAt, err)
	}
}
