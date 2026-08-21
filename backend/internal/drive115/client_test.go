package drive115

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func TestFolderPathUsesValidated115Breadcrumbs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Query().Get("cid") {
		case "20":
			_, _ = w.Write([]byte(`{"state":true,"path":[{"cid":"0","name":"Root"},{"cid":10,"name":"Media"},{"cid":"20","name":"Movies"}]}`))
		case "30":
			_, _ = w.Write([]byte(`{"state":true,"path":[{"cid":"0","name":"Root"},{"cid":"30","name":"bad/name"}]}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	client := NewClient("UID=uid; CID=cid; SEID=seid", time.Second)
	client.filesURL = server.URL
	path, err := client.FolderPath(context.Background(), "20")
	if err != nil || path != "Media/Movies" {
		t.Fatalf("path=%q err=%v", path, err)
	}
	if _, err := client.FolderPath(context.Background(), "30"); !errors.Is(err, ErrUpstreamResponse) {
		t.Fatalf("invalid breadcrumb error=%v", err)
	}
}

func TestEnsureFolderReusesExistingThenCreates(t *testing.T) {
	listCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Cookie") != "UID=uid; CID=cid; SEID=seid" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodGet:
			listCalls++
			if listCalls == 1 {
				_, _ = w.Write([]byte(`{"state":true,"count":1,"data":[{"cid":"88","n":"绿灯侠：绿灯长明","pid":"10"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"state":true,"count":0,"data":[]}`))
		case request.Method == http.MethodPost:
			_ = request.ParseForm()
			if request.FormValue("pid") != "10" || request.FormValue("cname") != "新片名" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"state":true,"cid":"99"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewClient("UID=uid; CID=cid; SEID=seid", time.Second)
	client.filesURL = server.URL
	client.folderAddURL = server.URL
	existing, err := client.EnsureFolder(context.Background(), "10", "绿灯侠：绿灯长明")
	if err != nil || existing != "88" {
		t.Fatalf("reuse=%q err=%v", existing, err)
	}
	created, err := client.EnsureFolder(context.Background(), "10", "新片名")
	if err != nil || created != "99" {
		t.Fatalf("create=%q err=%v", created, err)
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

func TestAddOfflineURLsUsesSignedWebJSONEndpoint(t *testing.T) {
	const cookie = "UID=uid; CID=cid; SEID=seid"
	infoCalls := 0
	offlineCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Cookie") != cookie {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/info":
			infoCalls++
			_, _ = w.Write([]byte(`{"sign":"signature","time":123}`))
		case "/offline":
			offlineCalls++
			if request.Method != http.MethodPost || request.URL.Query().Get("ct") != "lixian" || request.URL.Query().Get("ac") != "add_task_urls" || request.FormValue("wp_path_id") != "456" || request.FormValue("sign") != "signature" || request.FormValue("time") != "123" || request.FormValue("url[0]") != "magnet:?xt=urn:btih:one" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"state":true,"result":[{"info_hash":"one"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewClient(cookie, time.Second)
	client.offlineInfoURL = server.URL + "/info"
	client.offlineAddURL = server.URL + "/offline?ct=lixian&ac=add_task_urls"
	if err := client.AddOfflineURLs(context.Background(), "456", []string{"magnet:?xt=urn:btih:one"}); err != nil {
		t.Fatal(err)
	}
	if infoCalls != 1 || offlineCalls != 1 {
		t.Fatalf("infoCalls=%d offlineCalls=%d", infoCalls, offlineCalls)
	}
}

func TestAddOfflineURLsMarksMalformedJSONResponseUncertain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/info" {
			_, _ = w.Write([]byte(`{"sign":"signature","time":"123"}`))
			return
		}
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()
	client := NewClient("UID=uid; CID=cid; SEID=seid", time.Second)
	client.offlineInfoURL = server.URL + "/info"
	client.offlineAddURL = server.URL + "/offline"
	err := client.AddOfflineURLs(context.Background(), "456", []string{"magnet:?xt=urn:btih:one"})
	var writeErr *WriteError
	if !errors.As(err, &writeErr) || !writeErr.Uncertain || writeErr.Code != "invalid_response" {
		t.Fatalf("error = %#v, want uncertain invalid_response", err)
	}
}

func TestAddOfflineURLsMarksPartialResultUncertain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/info" {
			_, _ = w.Write([]byte(`{"sign":"signature","time":123}`))
			return
		}
		_, _ = w.Write([]byte(`{"state":true,"result":[]}`))
	}))
	defer server.Close()
	client := NewClient("UID=uid; CID=cid; SEID=seid", time.Second)
	client.offlineInfoURL = server.URL + "/info"
	client.offlineAddURL = server.URL + "/offline"
	err := client.AddOfflineURLs(context.Background(), "456", []string{"magnet:?xt=urn:btih:one"})
	var writeErr *WriteError
	if !errors.As(err, &writeErr) || !writeErr.Uncertain || writeErr.Code != "partial_result" {
		t.Fatalf("error = %#v, want uncertain partial_result", err)
	}
}
