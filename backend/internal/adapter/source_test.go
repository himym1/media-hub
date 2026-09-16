package adapter

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"media-hub/backend/internal/config"
	"media-hub/backend/internal/search"
)

type classifiedWriteError bool

func (e classifiedWriteError) Error() string               { return "classified write" }
func (e classifiedWriteError) AutomaticRetryAllowed() bool { return bool(e) }

func TestAutomaticWriteRetryClassification(t *testing.T) {
	if automaticWriteRetryAllowed(classifiedWriteError(false)) {
		t.Fatal("provider rejection remained automatically retryable")
	}
	if !automaticWriteRetryAllowed(classifiedWriteError(true)) || !automaticWriteRetryAllowed(http.ErrServerClosed) {
		t.Fatal("retryable or unclassified failure was blocked")
	}
}

func TestNewUsesBuiltinSourcesWhenURLEmpty(t *testing.T) {
	for _, id := range []string{"mikan", "sidhub", "pansou"} {
		source := New(config.SearchSource{ID: id}, time.Second, nil, nil)
		if source == nil || source.ID() != id {
			t.Fatalf("source %s = %#v", id, source)
		}
	}
}

func TestNewUsesNativeSidhubForOfficialHosts(t *testing.T) {
	for _, raw := range []string{"", "https://sidhub.cc", "https://www.sidhub.cc/", "https://seedog.cc", "https://www.seedog.cc/"} {
		got := New(config.SearchSource{ID: "sidhub", BaseURL: raw}, time.Second, nil, nil)
		if _, ok := got.(*Sidhub); !ok {
			t.Fatalf("source for %q = %#v", raw, got)
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
	frame, ok := New(config.SearchSource{ID: "framehdr", Account: "user", Token: "pass"}, time.Second, nil, proxyURL).(*FrameHDR)
	if !ok {
		t.Fatalf("source = %#v", frame)
	}
	request, _ = http.NewRequest(http.MethodGet, "https://framehdr.com/search.php?q=test", nil)
	transport = frame.client.Transport.(*http.Transport)
	resolved, err = transport.Proxy(request)
	if err != nil || resolved.String() != proxyURL.String() {
		t.Fatalf("proxy = %v, err = %v", resolved, err)
	}
	juying, ok := New(config.SearchSource{ID: "juying", Account: "app-id", Token: "app-key"}, time.Second, nil, proxyURL).(*Juying)
	if !ok {
		t.Fatalf("source = %#v", juying)
	}
	request, _ = http.NewRequest(http.MethodGet, "https://www.jying.top/api/dev/movies/", nil)
	transport = juying.client.Transport.(*http.Transport)
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

func TestNewUsesNativePansouWhenURLConfigured(t *testing.T) {
	empty := New(config.SearchSource{ID: "pansou"}, time.Second, nil, nil)
	if native, ok := empty.(*Pansou); !ok || native.baseURL != defaultPansouURL {
		t.Fatalf("default pansou = %#v", empty)
	}
	got := New(config.SearchSource{ID: "pansou", BaseURL: "http://172.17.0.1:57081", Token: "token"}, time.Second, nil, nil)
	native, ok := got.(*Pansou)
	if !ok || native.baseURL != "http://172.17.0.1:57081" || native.token != "token" {
		t.Fatalf("pansou = %#v", got)
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
func TestNewUsesNativeFrameHDROnlyWithCompleteOfficialCredentials(t *testing.T) {
	for _, source := range []config.SearchSource{
		{ID: "framehdr"},
		{ID: "framehdr", Account: "user"},
		{ID: "framehdr", Token: "pass"},
	} {
		if got := New(source, time.Second, nil, nil); got != nil {
			t.Fatalf("source = %#v", got)
		}
	}
	for _, raw := range []string{"", "https://framehdr.com", "https://www.framehdr.com/"} {
		got := New(config.SearchSource{ID: "framehdr", BaseURL: raw, Account: "user", Token: "pass"}, time.Second, nil, nil)
		if _, ok := got.(*FrameHDR); !ok {
			t.Fatalf("source for %q = %#v", raw, got)
		}
	}
	for _, raw := range []string{"http://framehdr.com", "https://framehdr.com/path", "https://framehdr.com:8443", "https://evil.example"} {
		got := New(config.SearchSource{ID: "framehdr", BaseURL: raw, Account: "user", Token: "pass"}, time.Second, nil, nil)
		if _, ok := got.(*search.HTTPSource); !ok {
			t.Fatalf("source for %q = %#v", raw, got)
		}
	}
}

func TestNewUsesNativeJuyingOnlyWithCompleteOfficialCredentials(t *testing.T) {
	for _, source := range []config.SearchSource{
		{ID: "juying"},
		{ID: "juying", Account: "app-id"},
		{ID: "juying", Token: "app-key"},
	} {
		if got := New(source, time.Second, nil, nil); got != nil {
			t.Fatalf("source = %#v", got)
		}
	}
	for _, raw := range []string{"", "https://jying.top", "https://www.jying.top/"} {
		got := New(config.SearchSource{ID: "juying", BaseURL: raw, Account: "app-id", Token: "app-key"}, time.Second, nil, nil)
		if _, ok := got.(*Juying); !ok {
			t.Fatalf("source for %q = %#v", raw, got)
		}
	}
	for _, raw := range []string{"http://jying.top", "https://jying.top/path", "https://jying.top:8443", "https://evil.example"} {
		got := New(config.SearchSource{ID: "juying", BaseURL: raw, Account: "app-id", Token: "app-key"}, time.Second, nil, nil)
		if _, ok := got.(*search.HTTPSource); !ok {
			t.Fatalf("source for %q = %#v", raw, got)
		}
	}
	web := New(config.SearchSource{ID: "juying", AuthMode: "web", Account: "user", Token: "pass"}, time.Second, nil, nil)
	if native, ok := web.(*Juying); !ok || native.authMode != "web" {
		t.Fatalf("web source = %#v", web)
	}
	legacy := New(config.SearchSource{ID: "juying", Account: "app-id", Token: "api-key"}, time.Second, nil, nil)
	if native, ok := legacy.(*Juying); !ok || native.authMode != "developer" {
		t.Fatalf("legacy source = %#v", legacy)
	}
	if got := New(config.SearchSource{ID: "juying", AuthMode: "invalid", Account: "user", Token: "pass"}, time.Second, nil, nil); got != nil {
		t.Fatalf("invalid mode source = %#v", got)
	}
}

func TestEnsureTransferDestinationWithoutEnsurerKeepsParent(t *testing.T) {
	id, title, err := ensureTransferDestination(context.Background(), nil, "movie-folder", "寻找艾米丽", "Finding.Emily")
	if err != nil || id != "movie-folder" || title != "寻找艾米丽" {
		t.Fatalf("id=%q title=%q err=%v", id, title, err)
	}
}

func TestStorageFolderNamePrefersMediaTitle(t *testing.T) {
	if got := storageFolderName("寻找艾米丽", "Finding.Emily.2026.mkv"); got != "寻找艾米丽" {
		t.Fatalf("title=%q", got)
	}
	if got := storageFolderName("", "Finding.Emily.2026.mkv"); got != "Finding.Emily.2026.mkv" {
		t.Fatalf("fallback=%q", got)
	}
}
