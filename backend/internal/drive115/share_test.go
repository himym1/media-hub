package drive115

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.FormValue("file_id") != "0" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"state":false,"errno":4100024}`))
	}))
	defer server.Close()
	client := NewClient("UID=123_session", time.Second)
	client.shareReceiveURL = server.URL
	if err := client.ReceiveShare(context.Background(), "0", "abc123", "", nil); err != nil {
		t.Fatal(err)
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
		if request.FormValue("file_id") != "0" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()
	client := NewClient("UID=123_session", time.Second)
	client.shareReceiveURL = server.URL
	err := client.ReceiveShare(context.Background(), "0", "abc123", "", nil)
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
		err := client.ReceiveShare(context.Background(), "0", "abc123", "", nil)
		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("cookie = %q error = %#v", cookie, err)
		}
	}
	if requests != 0 {
		t.Fatalf("requests = %d", requests)
	}
}

func TestReceiveShareUsesZeroToReceiveEntireShare(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests++
		if request.Method != http.MethodPost || request.FormValue("user_id") != "123" || request.FormValue("file_id") != "0" || request.FormValue("cid") != "456" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"state":true}`))
	}))
	defer server.Close()
	client := NewClient("UID=123_session", time.Second)
	client.shareReceiveURL = server.URL
	if err := client.ReceiveShare(context.Background(), "456", "abc123", "xy9z", nil); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d", requests)
	}
}

func TestShareVideoNamesListsNestedMediaWithoutReceiving(t *testing.T) {
	const cookie = "UID=123_session; CID=cid; SEID=seid"
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests++
		if request.Method != http.MethodGet || request.Header.Get("Cookie") != cookie || request.URL.Query().Get("share_code") != "abc123" || request.URL.Query().Get("receive_code") != "xy9z" || request.URL.Query().Get("offset") != "0" {
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
	names, err := client.ShareVideoNames(context.Background(), "abc123", "xy9z")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "Van.Helsing.2004.2160p.mkv" || requests != 2 {
		t.Fatalf("names=%#v requests=%d", names, requests)
	}
}

func TestShareVideoNamesRejectsInvalidInputBeforeRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	client := NewClient("UID=123_session", time.Second)
	client.shareSnapURL = server.URL
	if _, err := client.ShareVideoNames(context.Background(), "bad/code", ""); !errors.Is(err, ErrUpstreamResponse) {
		t.Fatalf("error = %#v", err)
	}
	if requests != 0 {
		t.Fatalf("requests = %d", requests)
	}
}
