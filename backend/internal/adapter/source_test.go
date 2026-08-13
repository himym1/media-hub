package adapter

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"media-hub/backend/internal/config"
	"media-hub/backend/internal/search"
)

func TestNewUsesBuiltinMikanWhenURLEmpty(t *testing.T) {
	source := New(config.SearchSource{ID: "mikan", Label: "蜜柑"}, time.Second, nil, nil)
	if source == nil || source.ID() != "mikan" {
		t.Fatalf("source = %#v", source)
	}
}

func TestNewPassesProxyOnlyToBuiltinMikan(t *testing.T) {
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
}

func TestNewDoesNotApplyMikanProxyToContractSource(t *testing.T) {
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

func TestNewIgnoresUnconfiguredNonMikanSource(t *testing.T) {
	if source := New(config.SearchSource{ID: "dian", Label: "点点"}, time.Second, nil, nil); source != nil {
		t.Fatalf("source = %#v", source)
	}
}
