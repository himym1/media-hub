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
			if request.URL.Query().Get("UserId") != "user-1" ||
				!strings.Contains(request.Header.Get("X-Emby-Authorization"), `UserId="user-1"`) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
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
		preview.FileCount != 2 || !preview.DeletesFiles || !preview.CloudKept || preview.VersionCount != 1 {
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

func TestDeleteItemUsesUserSessionToken(t *testing.T) {
	var deleted bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if serveUserSession(w, request) {
			return
		}
		switch {
		case request.URL.Path == "/Users/user-1/Items/item-1" && request.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/library/movie.strm"}`))
		case request.URL.Path == "/Items/item-1" && request.Method == http.MethodDelete:
			authorization := request.Header.Get("X-Emby-Authorization")
			if request.URL.Query().Get("Recursive") != "" || request.URL.Query().Get("UserId") != "user-1" ||
				request.Header.Get("X-Emby-Token") != "user-token" ||
				!strings.Contains(authorization, `UserId="user-1"`) ||
				!strings.Contains(authorization, `Token="user-token"`) {
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

	if err := deleteTestClient(server.URL).DeleteItem(context.Background(), "item-1"); err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("Emby delete was not called")
	}
}

func TestDeletePreviewRejectsLocalLeftoversIn115Library(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/Items/local-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"Id":"local-1","Name":"Local","Type":"Movie","ParentId":"library-1","Path":"/volume1/media/movie.mkv"}`))
	}))
	defer server.Close()
	client := NewClient(server.URL, "emby-key", time.Second)
	client.Configure(RuntimeConfig{BaseURL: server.URL, APIKey: "emby-key", MovieLibraryID: "library-1"})
	_, err := client.DeletePreview(context.Background(), "local-1")
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
	if preview.FileCount != 0 || !preview.DeletesFiles || !preview.CloudKept || preview.VersionCount != 1 {
		t.Fatalf("preview=%#v", preview)
	}
}

func TestDeleteItemFallsBackToPostDelete(t *testing.T) {
	var posted bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if serveUserSession(w, request) {
			return
		}
		switch {
		case request.URL.Path == "/Users/user-1/Items/item-1" && request.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/library/movie.strm"}`))
		case request.URL.Path == "/Items/item-1" && request.Method == http.MethodDelete:
			w.WriteHeader(http.StatusMethodNotAllowed)
		case request.URL.Path == "/Items/item-1/Delete" && request.Method == http.MethodPost:
			if request.URL.Query().Get("Recursive") != "" || request.Header.Get("X-Emby-Token") != "user-token" {
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
	if err := deleteTestClient(server.URL).DeleteItem(context.Background(), "item-1"); err != nil {
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
	if preview.FileCount != 0 || !preview.DeletesFiles || !preview.CloudKept || preview.VersionCount != 1 {
		t.Fatalf("preview=%#v", preview)
	}
}

func TestDeleteItemFallsBackToIdsDelete(t *testing.T) {
	var posted bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if serveUserSession(w, request) {
			return
		}
		switch {
		case request.URL.Path == "/Users/user-1/Items/item-1" && request.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/library/movie.strm"}`))
		case request.URL.Path == "/Items/item-1" && request.Method == http.MethodDelete:
			w.WriteHeader(http.StatusMethodNotAllowed)
		case request.URL.Path == "/Items/item-1/Delete" && request.Method == http.MethodPost:
			w.WriteHeader(http.StatusNotFound)
		case request.URL.Path == "/Items/Delete" && request.Method == http.MethodPost:
			if request.URL.Query().Get("Ids") != "item-1" || request.URL.Query().Get("UserId") != "user-1" ||
				request.Header.Get("X-Emby-Token") != "user-token" ||
				!strings.Contains(request.Header.Get("X-Emby-Authorization"), `UserId="user-1"`) {
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
	if err := deleteTestClient(server.URL).DeleteItem(context.Background(), "item-1"); err != nil {
		t.Fatal(err)
	}
	if !posted {
		t.Fatal("Emby Ids delete was not called")
	}
}

func TestDeleteItemMapsBadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if serveUserSession(w, request) {
			return
		}
		if request.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/library/movie.strm"}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	if err := deleteTestClient(server.URL).DeleteItem(context.Background(), "item-1"); !errors.Is(err, ErrDeleteRejected) {
		t.Fatalf("error=%v", err)
	}
}

func TestDeleteItemRejectsUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if serveUserSession(w, request) {
			return
		}
		if request.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/library/movie.strm"}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, "no")
	}))
	defer server.Close()
	if err := deleteTestClient(server.URL).DeleteItem(context.Background(), "item-1"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("error=%v", err)
	}
}

func TestDeleteItemRequiresUserPassword(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet &&
			(request.URL.Path == "/Items/item-1" || request.URL.Path == "/Users/user-1/Items/item-1") {
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","Path":"/library/movie.strm"}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	if err := NewClient(server.URL, "emby-key", time.Second).DeleteItem(context.Background(), "item-1"); !errors.Is(err, ErrDeleteNeedsUser) {
		t.Fatalf("error=%v", err)
	}
	if err := NewConfiguredClient(RuntimeConfig{
		BaseURL: server.URL, APIKey: "emby-key", UserID: "user-1",
	}, time.Second).DeleteItem(context.Background(), "item-1"); !errors.Is(err, ErrDeleteNeedsUser) {
		t.Fatalf("error=%v", err)
	}
}

func TestDeleteItemRemovesRelatedVersions(t *testing.T) {
	var deleted []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if serveUserSession(w, request) {
			return
		}
		switch {
		case request.URL.Path == "/Users/user-1/Items/item-1" && request.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","ProductionYear":2012,"ProviderIds":{"Tmdb":"1930"},"Path":"/library/a/movie.strm"}`))
		case request.URL.Path == "/Items" && request.Method == http.MethodGet:
			if request.URL.Query().Get("ParentId") != "lib-1" || request.URL.Query().Get("SearchTerm") != "Movie" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"Items":[
				{"Id":"item-1","Name":"Movie","Type":"Movie","ProductionYear":2012,"ProviderIds":{"Tmdb":"1930"},"Path":"/library/a/movie.strm"},
				{"Id":"item-2","Name":"Movie","Type":"Movie","ProductionYear":2012,"ProviderIds":{"Tmdb":"1930"},"Path":"/library/b/movie.strm"},
				{"Id":"item-3","Name":"Other","Type":"Movie","ProductionYear":2012,"ProviderIds":{"Tmdb":"999"},"Path":"/library/c/movie.strm"}
			]}`))
		case strings.HasPrefix(request.URL.Path, "/Items/item-") && request.Method == http.MethodDelete:
			deleted = append(deleted, strings.TrimPrefix(request.URL.Path, "/Items/"))
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewConfiguredClient(RuntimeConfig{
		BaseURL: server.URL, APIKey: "emby-key", UserID: "user-1", Password: "emby-pw", MovieLibraryID: "lib-1",
	}, time.Second)
	preview, err := client.DeletePreview(context.Background(), "item-1")
	if err != nil || preview.VersionCount != 2 {
		t.Fatalf("preview=%#v err=%v", preview, err)
	}
	if err := client.DeleteItem(context.Background(), "item-1"); err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 2 || deleted[0] != "item-1" || deleted[1] != "item-2" {
		t.Fatalf("deleted=%v", deleted)
	}
}

func TestDeleteItemFindsRelatedVersionsWithoutLibraryIDs(t *testing.T) {
	var deleted []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if serveUserSession(w, request) {
			return
		}
		switch {
		case request.URL.Path == "/Users/user-1/Items/item-1" && request.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"Id":"item-1","Name":"Movie","Type":"Movie","ProductionYear":2012,"ProviderIds":{"Tmdb":"1930"},"Path":"/library/a/movie.strm"}`))
		case request.URL.Path == "/Items" && request.Method == http.MethodGet:
			if request.URL.Query().Get("ParentId") != "" || request.URL.Query().Get("SearchTerm") != "Movie" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"Items":[
				{"Id":"item-1","Name":"Movie","Type":"Movie","ProductionYear":2012,"ProviderIds":{"Tmdb":"1930"},"Path":"/library/a/movie.strm"},
				{"Id":"item-2","Name":"Movie","Type":"Movie","ProductionYear":2012,"ProviderIds":{"Tmdb":"1930"},"Path":"/library/b/movie.strm"}
			]}`))
		case strings.HasPrefix(request.URL.Path, "/Items/item-") && request.Method == http.MethodDelete:
			deleted = append(deleted, strings.TrimPrefix(request.URL.Path, "/Items/"))
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewConfiguredClient(RuntimeConfig{
		BaseURL: server.URL, APIKey: "emby-key", UserID: "user-1", Password: "emby-pw",
	}, time.Second)
	preview, err := client.DeletePreview(context.Background(), "item-1")
	if err != nil || preview.VersionCount != 2 {
		t.Fatalf("preview=%#v err=%v", preview, err)
	}
	if err := client.DeleteItem(context.Background(), "item-1"); err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 2 || deleted[0] != "item-1" || deleted[1] != "item-2" {
		t.Fatalf("deleted=%v", deleted)
	}
}

func deleteTestClient(baseURL string) *Client {
	return NewConfiguredClient(RuntimeConfig{
		BaseURL: baseURL, APIKey: "emby-key", UserID: "user-1", Password: "emby-pw",
	}, time.Second)
}

func serveUserSession(w http.ResponseWriter, request *http.Request) bool {
	switch {
	case request.URL.Path == "/Users/user-1/Authenticate" && request.Method == http.MethodPost:
		var body struct {
			Pw string `json:"Pw"`
		}
		_ = json.NewDecoder(request.Body).Decode(&body)
		if body.Pw != "emby-pw" || request.Header.Get("X-Emby-Token") != "" {
			w.WriteHeader(http.StatusUnauthorized)
			return true
		}
		_, _ = w.Write([]byte(`{"AccessToken":"user-token"}`))
		return true
	case request.URL.Path == "/Users/user-1" && request.Method == http.MethodGet:
		_, _ = w.Write([]byte(`{"Id":"user-1","Name":"admin"}`))
		return true
	case request.URL.Path == "/Users/AuthenticateByName" && request.Method == http.MethodPost:
		_, _ = w.Write([]byte(`{"AccessToken":"user-token"}`))
		return true
	}
	return false
}
