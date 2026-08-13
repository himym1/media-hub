package adapter

import (
	"testing"
	"time"

	"media-hub/backend/internal/config"
	"media-hub/backend/internal/search"
)

func TestNewUsesBuiltinMikanWhenURLEmpty(t *testing.T) {
	source := New(config.SearchSource{ID: "mikan", Label: "蜜柑"}, time.Second, nil)
	if source == nil || source.ID() != "mikan" {
		t.Fatalf("source = %#v", source)
	}
}

func TestNewUsesHTTPSourceWhenContractURLConfigured(t *testing.T) {
	source := New(config.SearchSource{ID: "dian", Label: "点点", BaseURL: "https://adapter.example/dian"}, time.Second, nil)
	httpSource, ok := source.(*search.HTTPSource)
	if !ok || httpSource == nil {
		t.Fatalf("source = %#v", source)
	}
}

func TestNewIgnoresUnconfiguredNonMikanSource(t *testing.T) {
	if source := New(config.SearchSource{ID: "dian", Label: "点点"}, time.Second, nil); source != nil {
		t.Fatalf("source = %#v", source)
	}
}
