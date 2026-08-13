package drive115

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SheltonZhu/115driver/pkg/crypto/m115"
)

func TestStatusReadsNonIdentifyingAccountMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Cookie") != "UID=uid; CID=cid; SEID=seid" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"state":true,"data":{"space_info":{"all_use":{"size":12.75},"all_total":{"size":"100.9"}}}}`))
	}))
	defer server.Close()

	client := NewClient("UID=uid; CID=cid; SEID=seid", time.Second)
	client.userInfoURL = server.URL
	status, err := client.Status(context.Background())
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !status.Authorized || status.UsedBytes != 12 || status.TotalBytes != 100 {
		t.Fatalf("unexpected status: %#v", status)
	}
}

func TestListFilesMapsWebFoldersAndFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("cid") != "0" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"state":true,"count":2,"data":[{"cid":"10","pid":"0","n":"Movies"},{"fid":"20","cid":"10","n":"a.mkv","s":8,"t":1700000000}]}`))
	}))
	defer server.Close()
	client := NewClient("UID=uid; CID=cid; SEID=seid", time.Second)
	client.filesURL = server.URL
	items, total, err := client.ListFiles(context.Background(), "0", 50, 0)
	if err != nil || total != 2 || len(items) != 2 {
		t.Fatalf("items=%#v total=%d err=%v", items, total, err)
	}
	if items[0].Kind != "folder" || items[0].ID != "10" || items[1].Kind != "file" || items[1].ID != "20" || items[1].ParentID != "10" {
		t.Fatalf("unexpected items: %#v", items)
	}
}

func TestExecuteRenameUsesWebBatchRenameForm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/files/batch_rename" || request.Header.Get("Cookie") != "UID=uid; CID=cid; SEID=seid" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if request.FormValue("fid") != "20" || request.FormValue("file_name") != "renamed.mkv" || request.FormValue("files_new_name[20]") != "renamed.mkv" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"state":true}`))
	}))
	defer server.Close()
	client := NewClient("UID=uid; CID=cid; SEID=seid", time.Second)
	client.fileRenameURL = server.URL + "/files/batch_rename"
	if err := client.ExecuteFileCommand(context.Background(), "rename", map[string]any{"fileId": "20", "name": "renamed.mkv"}); err != nil {
		t.Fatalf("rename: %v", err)
	}
}

func TestOfflineRequestPayloadIncludesRequiredEncryptedFields(t *testing.T) {
	payload, err := offlineRequestPayload(123, "456", []string{"magnet:?xt=urn:btih:one", "https://example.test/file"})
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]string
	if err := json.Unmarshal(payload, &values); err != nil {
		t.Fatal(err)
	}
	if values["ac"] != "add_task_urls" || values["uid"] != "123" || values["wp_path_id"] != "456" || values["app_ver"] != offlineAppVersion || values["url[0]"] != "magnet:?xt=urn:btih:one" || values["url[1]"] != "https://example.test/file" {
		t.Fatalf("unexpected payload: %#v", values)
	}
}

func TestAddOfflineURLsUsesOneCookieSnapshotAndRejectsMalformedEncryptedResponse(t *testing.T) {
	const cookie = "UID=uid; CID=cid; SEID=seid"
	profileCalls := 0
	offlineCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Cookie") != cookie {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/profile":
			profileCalls++
			_, _ = w.Write([]byte(`{"state":true,"data":{"user_id":123}}`))
		case "/offline":
			offlineCalls++
			if request.Method != http.MethodPost || request.FormValue("data") == "" || request.URL.Query().Get("t") == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"state":true,"data":"not-base64"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewClient(cookie, time.Second)
	client.userProfileURL = server.URL + "/profile"
	client.offlineAddURL = server.URL + "/offline?ac=add_task_urls"
	err := client.AddOfflineURLs(context.Background(), "456", []string{"magnet:?xt=urn:btih:one"})
	var writeErr *WriteError
	if !errors.As(err, &writeErr) || !writeErr.Uncertain || writeErr.Code != "invalid_response" {
		t.Fatalf("error = %#v, want uncertain invalid_response", err)
	}
	if profileCalls != 1 || offlineCalls != 1 {
		t.Fatalf("profileCalls=%d offlineCalls=%d", profileCalls, offlineCalls)
	}
}

func TestDecodeOfflineResponseRejectsMalformedInputWithoutPanic(t *testing.T) {
	if _, err := decodeOfflineResponse("not-base64", m115.Key{}); !errors.Is(err, ErrUpstreamResponse) {
		t.Fatalf("error = %v, want ErrUpstreamResponse", err)
	}
}
