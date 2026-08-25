package assrt

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

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

func TestAssrtTransportFallsBackOnBadGateway(t *testing.T) {
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":0,"sub":{"subs":[]}}`))
	}))
	defer mirror.Close()
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer primary.Close()

	transport := &assrtTransport{
		primary: rewriteHost(primary.URL),
		mirror:  rewriteHost(mirror.URL),
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
