package emby

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDeletePreviewOmitsFilesystemPathsAndKeepsCloud(t *testing.T) {
	var sawToken bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != "emby-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		sawToken = true
		switch request.URL.Path {
		case "/Users/user-1/Items/item-1":
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"验收影片","Type":"Movie","Path":"/private/movie.strm","MediaSources":[{"Path":"/private/movie.strm"}]}`))
		case "/Items/item-1/DeleteInfo":
			_, _ = w.Write([]byte(`{"Paths":["/private/movie.strm","/private/movie.nfo"],"LocalFileCount":2}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	preview, err := NewClient(server.URL, "emby-key", time.Second, "user-1").DeletePreview(context.Background(), "item-1")
	if err != nil {
		t.Fatal(err)
	}
	if !sawToken || preview.ID != "item-1" || preview.Name != "验收影片" || preview.Type != "Movie" ||
		preview.FileCount != 2 || !preview.DeletesFiles || !preview.CloudKept {
		t.Fatalf("preview=%#v", preview)
	}
	encoded, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "/private") || strings.Contains(string(encoded), ".strm") {
		t.Fatalf("preview leaked path: %s", encoded)
	}
}

func TestDeleteItemUsesRecursiveDeleteWithoutExposingPaths(t *testing.T) {
	var deleted bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/Items/item-1" && request.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/library/movie.strm"}`))
		case request.URL.Path == "/Items/item-1" && request.Method == http.MethodDelete:
			if request.URL.Query().Get("Recursive") != "true" || request.Header.Get("X-Emby-Token") != "emby-key" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	if err := NewClient(server.URL, "emby-key", time.Second).DeleteItem(context.Background(), "item-1"); err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("Emby delete was not called")
	}
}

func TestDeletePreviewRejectsLocalOnlyItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/Items/local-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"Id":"local-1","Name":"Local","Type":"Movie","Path":"/volume1/media/movie.mkv"}`))
	}))
	defer server.Close()
	_, err := NewClient(server.URL, "emby-key", time.Second).DeletePreview(context.Background(), "local-1")
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("error=%v", err)
	}
}

func TestDeletePreviewIgnoresUnusableDeleteInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/Items/item-1" {
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/library/movie.strm"}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	preview, err := NewClient(server.URL, "emby-key", time.Second).DeletePreview(context.Background(), "item-1")
	if err != nil {
		t.Fatal(err)
	}
	if preview.FileCount != 0 || !preview.DeletesFiles || !preview.CloudKept {
		t.Fatalf("preview=%#v", preview)
	}
}

func TestDeleteItemFallsBackToPostDelete(t *testing.T) {
	var posted bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/Items/item-1" && request.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/library/movie.strm"}`))
		case request.URL.Path == "/Items/item-1" && request.Method == http.MethodDelete:
			w.WriteHeader(http.StatusMethodNotAllowed)
		case request.URL.Path == "/Items/item-1/Delete" && request.Method == http.MethodPost:
			if request.URL.Query().Get("Recursive") != "true" || request.Header.Get("X-Emby-Token") != "emby-key" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			posted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	if err := NewClient(server.URL, "emby-key", time.Second).DeleteItem(context.Background(), "item-1"); err != nil {
		t.Fatal(err)
	}
	if !posted {
		t.Fatal("Emby POST delete was not called")
	}
}

func TestDeletePreviewAllowsMissingDeleteInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/Items/item-1" {
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/library/movie.strm"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	preview, err := NewClient(server.URL, "emby-key", time.Second).DeletePreview(context.Background(), "item-1")
	if err != nil {
		t.Fatal(err)
	}
	if preview.FileCount != 0 || !preview.DeletesFiles || !preview.CloudKept {
		t.Fatalf("preview=%#v", preview)
	}
}

func TestDeleteItemRejectsUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/library/movie.strm"}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, "no")
	}))
	defer server.Close()
	if err := NewClient(server.URL, "emby-key", time.Second).DeleteItem(context.Background(), "item-1"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("error=%v", err)
	}
}
