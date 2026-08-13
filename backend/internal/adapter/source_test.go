package adapter

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"media-hub/backend/internal/config"
	"media-hub/backend/internal/search"
)

func TestNewUsesBuiltinSourcesWhenURLEmpty(t *testing.T) {
	for _, id := range []string{"mikan", "sidhub"} {
		source := New(config.SearchSource{ID: id}, time.Second, nil, nil)
		if source == nil || source.ID() != id {
			t.Fatalf("source %s = %#v", id, source)
		}
	}
}

func TestNewPassesProxyOnlyToBuiltinSources(t *testing.T) {
	proxyURL, err := url.Parse("http://proxy.local:8080")
	if err != nil {
		t.Fatal(err)
	}
	source, ok := New(config.SearchSource{ID: "mikan", Label: "蜜柑"}, time.Second, nil, proxyURL).(*Mikan)
	if !ok {
		t.Fatalf("source = %#v", source)
	}
	request, err := http.NewRequest(http.MethodGet, "https://mikanani.me/RSS/Search", nil)
	if err != nil {
		t.Fatal(err)
	}
	transport := source.client.Transport.(*http.Transport)
	resolved, err := transport.Proxy(request)
	if err != nil || resolved.String() != proxyURL.String() {
		t.Fatalf("proxy = %v, err = %v", resolved, err)
	}
	sidhub, ok := New(config.SearchSource{ID: "sidhub", Label: "Sidhub"}, time.Second, nil, proxyURL).(*Sidhub)
	if !ok {
		t.Fatalf("source = %#v", sidhub)
	}
	request, _ = http.NewRequest(http.MethodGet, "https://sidhub.cc/s/test/", nil)
	transport = sidhub.client.Transport.(*http.Transport)
	resolved, err = transport.Proxy(request)
	if err != nil || resolved.String() != proxyURL.String() {
		t.Fatalf("proxy = %v, err = %v", resolved, err)
	}
}

func TestNewDoesNotApplyBuiltinProxyToContractSource(t *testing.T) {
	proxyURL, err := url.Parse("http://proxy.local:8080")
	if err != nil {
		t.Fatal(err)
	}
	source := New(config.SearchSource{ID: "dian", Label: "点点", BaseURL: "https://adapter.example/dian"}, time.Second, nil, proxyURL)
	httpSource, ok := source.(*search.HTTPSource)
	if !ok || httpSource == nil {
		t.Fatalf("source = %#v", source)
	}
}

func TestNewIgnoresUnconfiguredContractSource(t *testing.T) {
	if source := New(config.SearchSource{ID: "dian", Label: "点点"}, time.Second, nil, nil); source != nil {
		t.Fatalf("source = %#v", source)
	}
}

func TestNewUsesContractAdapterForNonOfficialSidhubHost(t *testing.T) {
	source := New(config.SearchSource{ID: "sidhub", Label: "Sidhub", BaseURL: "https://adapter.example/sidhub"}, time.Second, nil, nil)
	if _, ok := source.(*search.HTTPSource); !ok {
		t.Fatalf("source = %#v", source)
	}
	for _, raw := range []string{"http://sidhub.cc", "https://sidhub.cc/path", "https://sidhub.cc:8443", "https://evil.example"} {
		source := New(config.SearchSource{ID: "sidhub", Label: "Sidhub", BaseURL: raw}, time.Second, nil, nil)
		if _, ok := source.(*search.HTTPSource); !ok {
			t.Fatalf("source for %q = %#v", raw, source)
		}
	}
}
