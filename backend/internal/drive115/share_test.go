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
	const cookie = "UID=uid; CID=cid; SEID=seid"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Cookie") != cookie || request.Header.Get("User-Agent") != userAgent {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/profile":
			_, _ = w.Write([]byte(`{"state":true,"data":{"user_id":123}}`))
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
	client.userProfileURL = server.URL + "/profile"
	client.shareReceiveURL = server.URL + "/receive"
	if err := client.ReceiveShare(context.Background(), "456", "abc123", "xy9z", []string{"10", "20", "10"}); err != nil {
		t.Fatal(err)
	}
}

func TestReceiveShareTreatsAlreadyReceivedAsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/profile":
			_, _ = w.Write([]byte(`{"state":true,"data":{"user_id":123}}`))
		case "/snap":
			_, _ = w.Write([]byte(`{"state":true,"data":{"count":1,"list":[{"fid":"10","cid":"0","fc":1}]}}`))
		case "/receive":
			_, _ = w.Write([]byte(`{"state":false,"errno":4100024}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewClient("UID=uid", time.Second)
	client.userProfileURL = server.URL + "/profile"
	client.shareSnapURL = server.URL + "/snap"
	client.shareReceiveURL = server.URL + "/receive"
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
		switch request.URL.Path {
		case "/profile":
			_, _ = w.Write([]byte(`{"state":true,"data":{"user_id":123}}`))
		case "/snap":
			_, _ = w.Write([]byte(`{"state":true,"data":{"count":1,"list":[{"fid":"10","cid":"0","fc":1}]}}`))
		case "/receive":
			_, _ = w.Write([]byte(`not-json`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewClient("UID=uid", time.Second)
	client.userProfileURL = server.URL + "/profile"
	client.shareSnapURL = server.URL + "/snap"
	client.shareReceiveURL = server.URL + "/receive"
	err := client.ReceiveShare(context.Background(), "0", "abc123", "", nil)
	var writeErr *WriteError
	if !errors.As(err, &writeErr) || !writeErr.Uncertain || writeErr.Code != "invalid_response" {
		t.Fatalf("error = %#v", err)
	}
}

func TestReceiveShareRejectsMissingUserIDBeforeSubmission(t *testing.T) {
	shareCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/profile" {
			_, _ = w.Write([]byte(`{"state":true,"data":{"user_id":0}}`))
			return
		}
		shareCalls++
		_, _ = w.Write([]byte(`{"state":true}`))
	}))
	defer server.Close()
	client := NewClient("UID=uid", time.Second)
	client.userProfileURL = server.URL + "/profile"
	client.shareReceiveURL = server.URL + "/receive"
	err := client.ReceiveShare(context.Background(), "0", "abc123", "", nil)
	if !errors.Is(err, ErrUnauthorized) || shareCalls != 0 {
		t.Fatalf("error = %#v share calls = %d", err, shareCalls)
	}
}

func TestReceiveShareResolvesAllRootItemsBeforeSubmission(t *testing.T) {
	snapCalls := 0
	receiveCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/profile":
			_, _ = w.Write([]byte(`{"state":true,"data":{"user_id":123}}`))
		case "/snap":
			snapCalls++
			switch request.URL.Query().Get("offset") {
			case "0":
				_, _ = w.Write([]byte(`{"state":true,"data":{"count":"3","list":[{"fid":"10","cid":"0","fc":"1"},{"fid":"0","cid":20,"fc":0}]}}`))
			case "2":
				_, _ = w.Write([]byte(`{"state":true,"data":{"count":3,"list":[{"fid":30,"cid":"0","fc":1}]}}`))
			default:
				w.WriteHeader(http.StatusBadRequest)
			}
		case "/receive":
			receiveCalls++
			if request.FormValue("user_id") != "123" || request.FormValue("file_id") != "10,20,30" || request.FormValue("cid") != "456" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"state":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewClient("UID=uid", time.Second)
	client.userProfileURL = server.URL + "/profile"
	client.shareSnapURL = server.URL + "/snap"
	client.shareReceiveURL = server.URL + "/receive"
	if err := client.ReceiveShare(context.Background(), "456", "abc123", "xy9z", nil); err != nil {
		t.Fatal(err)
	}
	if snapCalls != 2 || receiveCalls != 1 {
		t.Fatalf("snap calls = %d receive calls = %d", snapCalls, receiveCalls)
	}
}

func TestReceiveShareRejectsIncompleteSnapshotBeforeSubmission(t *testing.T) {
	receiveCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/profile":
			_, _ = w.Write([]byte(`{"state":true,"data":{"user_id":123}}`))
		case "/snap":
			if request.URL.Query().Get("offset") == "0" {
				_, _ = w.Write([]byte(`{"state":true,"data":{"count":2,"list":[{"fid":"10","cid":"0","fc":1}]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"state":true,"data":{"count":2,"list":[]}}`))
		case "/receive":
			receiveCalls++
			_, _ = w.Write([]byte(`{"state":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewClient("UID=uid", time.Second)
	client.userProfileURL = server.URL + "/profile"
	client.shareSnapURL = server.URL + "/snap"
	client.shareReceiveURL = server.URL + "/receive"
	err := client.ReceiveShare(context.Background(), "456", "abc123", "xy9z", nil)
	if !errors.Is(err, ErrUpstreamResponse) || receiveCalls != 0 {
		t.Fatalf("error = %#v receive calls = %d", err, receiveCalls)
	}
}
