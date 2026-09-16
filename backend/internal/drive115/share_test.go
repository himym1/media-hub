package drive115

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWriteErrorRejectsAutomaticRetryForDefinitiveFailures(t *testing.T) {
	for _, code := range []string{"provider_rejected", "invalid_request"} {
		if (&WriteError{Code: code, Err: ErrUpstreamResponse}).AutomaticRetryAllowed() {
			t.Fatalf("code %q remained automatically retryable", code)
		}
	}
	if !(&WriteError{Code: "temporary", Err: ErrUpstreamResponse}).AutomaticRetryAllowed() {
		t.Fatal("temporary failure was blocked")
	}
}

func TestReceiveShareSubmitsValidatedForm(t *testing.T) {
	const cookie = "UID=123_session; CID=cid; SEID=seid"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Cookie") != cookie || request.Header.Get("User-Agent") != userAgent {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/receive":
			if request.Method != http.MethodPost || request.FormValue("user_id") != "123" || request.FormValue("share_code") != "abc123" || request.FormValue("receive_code") != "xy9z" || request.FormValue("cid") != "456" || request.FormValue("file_id") != "10,20" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"state":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(cookie, time.Second)
	client.shareReceiveURL = server.URL + "/receive"
	if err := client.ReceiveShare(context.Background(), "456", "abc123", "xy9z", []string{"10", "20", "10"}); err != nil {
		t.Fatal(err)
	}
}

func TestReceiveShareTreatsAlreadyReceivedAsSuccess(t *testing.T) {
	for _, errno := range []string{"4100024", "4000023"} {
		t.Run(errno, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.FormValue("file_id") != "10" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				_, _ = w.Write([]byte(`{"state":false,"errno":` + errno + `}`))
			}))
			defer server.Close()
			client := NewClient("UID=123_session", time.Second)
			client.shareReceiveURL = server.URL
			if err := client.ReceiveShare(context.Background(), "0", "abc123", "", []string{"10"}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReceiveShareRejectsInvalidInputBeforeRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"state":true}`))
	}))
	defer server.Close()
	client := NewClient("UID=uid", time.Second)
	client.shareReceiveURL = server.URL
	for _, test := range []struct {
		destination string
		share       string
		code        string
		files       []string
	}{
		{"../1", "abc123", "", nil},
		{"1", "bad/code", "", nil},
		{"1", "abc123", "too-long-code", nil},
		{"1", "abc123", "", []string{"bad"}},
	} {
		var writeErr *WriteError
		err := client.ReceiveShare(context.Background(), test.destination, test.share, test.code, test.files)
		if !errors.As(err, &writeErr) || writeErr.Code != "invalid_request" {
			t.Fatalf("error = %#v", err)
		}
	}
	if calls != 0 {
		t.Fatalf("requests = %d", calls)
	}
}

func TestReceiveShareMarksMalformedResponseUncertain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.FormValue("file_id") != "10" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()
	client := NewClient("UID=123_session", time.Second)
	client.shareReceiveURL = server.URL
	err := client.ReceiveShare(context.Background(), "0", "abc123", "", []string{"10"})
	var writeErr *WriteError
	if !errors.As(err, &writeErr) || !writeErr.Uncertain || writeErr.Code != "invalid_response" {
		t.Fatalf("error = %#v", err)
	}
}

func TestReceiveShareRejectsInvalidCookieUserIDBeforeSubmission(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{"state":true}`))
	}))
	defer server.Close()

	for _, cookie := range []string{"CID=cid; SEID=seid", "UID=invalid_session; CID=cid; SEID=seid"} {
		client := NewClient(cookie, time.Second)
		client.shareReceiveURL = server.URL + "/receive"
		err := client.ReceiveShare(context.Background(), "0", "abc123", "", []string{"10"})
		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("cookie = %q error = %#v", cookie, err)
		}
	}
	if requests != 0 {
		t.Fatalf("requests = %d", requests)
	}
}

func TestReceiveShareInspectsRootIDsToReceiveEntireShare(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests++
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/snap" && request.URL.Query().Get("cid") == "":
			_, _ = w.Write([]byte(`{"state":true,"data":{"list":[{"fid":"10","fc":1,"n":"Van.Helsing.2004.mkv"},{"cid":"20","fc":0,"n":"Extras"}]}}`))
		case request.Method == http.MethodGet && request.URL.Path == "/snap" && request.URL.Query().Get("cid") == "20":
			_, _ = w.Write([]byte(`{"state":true,"data":{"list":[{"fid":"30","fc":1,"n":"sample.mp4"}]}}`))
		case request.Method == http.MethodPost && request.URL.Path == "/receive":
			if request.FormValue("user_id") != "123" || request.FormValue("file_id") != "10,20" || request.FormValue("cid") != "456" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"state":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewClient("UID=123_session", time.Second)
	client.shareSnapURL = server.URL + "/snap"
	client.shareReceiveURL = server.URL + "/receive"
	if err := client.ReceiveShare(context.Background(), "456", "abc123", "xy9z", nil); err != nil {
		t.Fatal(err)
	}
	if requests != 3 {
		t.Fatalf("requests = %d", requests)
	}
}

func TestInspectShareListsNestedMediaAndRootIDsWithoutReceiving(t *testing.T) {
	const cookie = "UID=123_session; CID=cid; SEID=seid"
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests++
		if request.Method != http.MethodGet || request.Header.Get("Cookie") != "" || request.URL.Query().Get("share_code") != "abc123" || request.URL.Query().Get("receive_code") != "xy9z" || request.URL.Query().Get("offset") != "0" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch request.URL.Query().Get("cid") {
		case "":
			_, _ = w.Write([]byte(`{"state":true,"data":{"count":2,"list":[{"cid":"10","fc":0,"n":"Movie"},{"fid":"11","fc":1,"n":"README.txt"}]}}`))
		case "10":
			_, _ = w.Write([]byte(`{"state":true,"data":{"list":[{"fid":"20","fc":1,"n":"Van.Helsing.2004.2160p.mkv"},{"fid":"21","fc":1,"n":"poster.jpg"}]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(cookie, time.Second)
	client.shareSnapURL = server.URL
	names, rootIDs, err := client.InspectShare(context.Background(), "abc123", "xy9z")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "Van.Helsing.2004.2160p.mkv" || len(rootIDs) != 2 || rootIDs[0] != "10" || rootIDs[1] != "11" || requests != 2 {
		t.Fatalf("names=%#v roots=%#v requests=%d", names, rootIDs, requests)
	}
}

func TestInspectShareFallsBackToSessionWhenAnonymousSnapRejected(t *testing.T) {
	const cookie = "UID=123_session; CID=cid; SEID=seid"
	anonymous := 0
	authed := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Cookie") == "" {
			anonymous++
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		authed++
		if request.Header.Get("Cookie") != cookie {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"state":true,"data":{"list":[{"fid":"20","fc":1,"n":"Van.Helsing.2004.mkv"}]}}`))
	}))
	defer server.Close()

	client := NewClient(cookie, time.Second)
	client.shareSnapURL = server.URL
	names, rootIDs, err := client.InspectShare(context.Background(), "abc123", "xy9z")
	if err != nil {
		t.Fatal(err)
	}
	if anonymous != 1 || authed != 1 || len(names) != 1 || names[0] != "Van.Helsing.2004.mkv" || len(rootIDs) != 1 || rootIDs[0] != "20" {
		t.Fatalf("anonymous=%d authed=%d names=%#v roots=%#v", anonymous, authed, names, rootIDs)
	}
}

func TestInspectShareRejectsInvalidInputBeforeRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	client := NewClient("UID=123_session", time.Second)
	client.shareSnapURL = server.URL
	if _, _, err := client.InspectShare(context.Background(), "bad/code", ""); !errors.Is(err, ErrUpstreamResponse) {
		t.Fatalf("error = %#v", err)
	}
	if requests != 0 {
		t.Fatalf("requests = %d", requests)
	}
}
