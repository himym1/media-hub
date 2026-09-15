package assrt

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestAssrtFileHostDetection(t *testing.T) {
	if !isAssrtFileHost("file1.assrt.net") || !isAssrtFileHost("file0.makedie.me") {
		t.Fatal("expected Assrt file hosts")
	}
	if isAssrtFileHost("api.assrt.net") || isAssrtFileHost("example.com") {
		t.Fatal("API hosts are not file CDNs")
	}
}

func TestAssrtMirrorHostMapping(t *testing.T) {
	if got := makedieHost("api.assrt.net"); got != "api.makedie.me" {
		t.Fatalf("makedieHost = %q", got)
	}
	if got := makedieHost("file0.assrt.net"); got != "file0.makedie.me" {
		t.Fatalf("file host = %q", got)
	}
	if got := assrtHost("api.makedie.me"); got != "api.assrt.net" {
		t.Fatalf("assrtHost = %q", got)
	}
	endpoint, _ := url.Parse("https://file0.assrt.net/onthefly/1.ass")
	rewriteAssrtURLToMakedie(endpoint)
	if endpoint.Host != "file0.makedie.me" {
		t.Fatalf("rewritten = %q", endpoint.Host)
	}
}

func TestAssrtTransportPrefersMirror(t *testing.T) {
	var hosts []string
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":0,"sub":{"subs":[]}}`))
	}))
	defer ok.Close()
	transport := &assrtTransport{
		primary: recordingTripper{hosts: &hosts, next: failingRoundTripper{}},
		mirror:  recordingTripper{hosts: &hosts, next: rewriteHost(ok.URL)},
	}
	request, err := http.NewRequest(http.MethodGet, "https://api.assrt.net/v1/sub/search?q=assrt", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || string(body) != `{"status":0,"sub":{"subs":[]}}` {
		t.Fatalf("status=%d body=%q err=%v", response.StatusCode, body, err)
	}
	if len(hosts) != 1 || hosts[0] != "api.makedie.me" {
		t.Fatalf("hosts=%v", hosts)
	}
}

func TestAssrtTransportFallsBackToOfficialWhenMirrorFails(t *testing.T) {
	var hosts []string
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ok.Close()
	transport := &assrtTransport{
		primary: recordingTripper{hosts: &hosts, next: rewriteHost(ok.URL)},
		mirror:  recordingTripper{hosts: &hosts, next: failingRoundTripper{}},
	}
	request, err := http.NewRequest(http.MethodGet, "https://api.assrt.net/v1/sub/search?q=assrt", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := transport.RoundTrip(request)
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("status=%v err=%v", response, err)
	}
	response.Body.Close()
	if len(hosts) < 2 || hosts[0] != "api.makedie.me" || hosts[1] != "api.assrt.net" {
		t.Fatalf("hosts=%v", hosts)
	}
}

func TestAssrtFileDownloadsIgnoreSourceProxy(t *testing.T) {
	proxyURL, err := url.Parse("http://127.0.0.1:9")
	if err != nil {
		t.Fatal(err)
	}
	client := newAssrtHTTPClient(time.Second, proxyURL)
	transport, ok := client.Transport.(*assrtTransport)
	if !ok {
		t.Fatal("expected assrt transport")
	}
	if transport.files != transport.primary || transport.filesMirror != transport.mirror {
		t.Fatal("Assrt file downloads must not use the source proxy")
	}
}

func TestAssrtTransportSendsFilesThroughDedicatedTripper(t *testing.T) {
	var fileHosts, apiHosts []string
	files := recordingTripper{hosts: &fileHosts, next: rewriteHost(httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).URL)}
	api := recordingTripper{hosts: &apiHosts, next: rewriteHost(httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).URL)}
	transport := &assrtTransport{primary: api, mirror: api, files: files, filesMirror: files}

	fileRequest, err := http.NewRequest(http.MethodGet, "http://file1.assrt.net/x.ass", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := transport.RoundTrip(fileRequest)
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("file status=%v err=%v", response, err)
	}
	response.Body.Close()

	apiRequest, err := http.NewRequest(http.MethodGet, "https://api.assrt.net/v1/sub/search?q=assrt", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err = transport.RoundTrip(apiRequest)
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("api status=%v err=%v", response, err)
	}
	response.Body.Close()
	if len(fileHosts) == 0 || len(apiHosts) == 0 {
		t.Fatalf("fileHosts=%v apiHosts=%v", fileHosts, apiHosts)
	}
}

func TestAssrtTransportFallsBackOnPrimaryError(t *testing.T) {
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mirror.Close()
	transport := &assrtTransport{
		primary: failingRoundTripper{},
		mirror:  rewriteHost(mirror.URL),
	}
	request, err := http.NewRequest(http.MethodGet, "https://file0.assrt.net/x.ass", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := transport.RoundTrip(request)
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("status=%v err=%v", response, err)
	}
	response.Body.Close()
}

type recordingTripper struct {
	hosts *[]string
	next  http.RoundTripper
}

func (t recordingTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL != nil {
		*t.hosts = append(*t.hosts, request.URL.Hostname())
	}
	return t.next.RoundTrip(request)
}

type failingRoundTripper struct{}

func (failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("tls handshake reset")
}

func rewriteHost(raw string) http.RoundTripper {
	target, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return hostRewrite{base: http.DefaultTransport, host: target.Host, scheme: target.Scheme}
}

type hostRewrite struct {
	base   http.RoundTripper
	host   string
	scheme string
}

func (t hostRewrite) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.URL.Scheme = t.scheme
	clone.URL.Host = t.host
	clone.Host = t.host
	return t.base.RoundTrip(clone)
}
