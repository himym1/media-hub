package drive115

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestInitSampleUploadReturnsHTTPSTicket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Cookie") != "UID=1001_session; CID=cid; SEID=seid" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(request.Body)
		form, _ := url.ParseQuery(string(body))
		if form.Get("userid") != "1001" || form.Get("filename") != "clip.ts" || form.Get("filesize") != "8" || form.Get("target") != "U_1_300" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"host":"http://bucket.oss-cn-shenzhen.aliyuncs.com","object":"A1/obj","accessid":"id","policy":"p","signature":"s","callback":"cb"}`))
	}))
	defer server.Close()
	client := NewClient("UID=1001_session; CID=cid; SEID=seid", time.Second)
	client.sampleInitURL = server.URL
	ticket, err := client.InitSampleUpload(context.Background(), "300", "clip.ts", 8)
	if err != nil {
		t.Fatal(err)
	}
	if ticket.Host != "https://bucket.oss-cn-shenzhen.aliyuncs.com" || ticket.Object != "A1/obj" || ticket.Target != "U_1_300" || ticket.Filename != "clip.ts" {
		t.Fatalf("ticket=%#v", ticket)
	}
}

func TestInitSampleUploadRejectsBadHostAndName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"host":"https://evil.example","object":"o","accessid":"id","policy":"p","signature":"s","callback":"cb"}`))
	}))
	defer server.Close()
	client := NewClient("UID=1001_session; CID=cid; SEID=seid", time.Second)
	client.sampleInitURL = server.URL
	if _, err := client.InitSampleUpload(context.Background(), "300", "clip.ts", 8); err == nil {
		t.Fatal("expected host rejection")
	}
	if _, err := client.InitSampleUpload(context.Background(), "300", "../clip.ts", 8); err == nil {
		t.Fatal("expected name rejection")
	}
	if _, err := client.InitSampleUpload(context.Background(), "300", "clip.txt", 8); err == nil {
		t.Fatal("expected extension rejection")
	}
}

func TestSanitizeOSSPostURL(t *testing.T) {
	host, err := sanitizeOSSPostURL("bucket.oss-cn-shenzhen.aliyuncs.com")
	if err != nil || host != "https://bucket.oss-cn-shenzhen.aliyuncs.com" {
		t.Fatalf("host=%q err=%v", host, err)
	}
	if _, err := sanitizeOSSPostURL("https://example.com"); err == nil {
		t.Fatal("expected rejection")
	}
}
