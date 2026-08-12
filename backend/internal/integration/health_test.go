package integration

import (
	"context"
	"testing"
)

type checkerStub struct {
	health Health
}

func (stub checkerStub) Check(context.Context) Health {
	return stub.health
}

func TestOverviewServicePreservesCheckerOrder(t *testing.T) {
	service := NewOverviewService(
		checkerStub{health: Health{ID: "sources", Status: StatusUnconfigured}},
		checkerStub{health: Health{ID: "emby", Status: StatusHealthy}},
	)

	results := service.Overview(context.Background())
	if len(results) != 2 {
		t.Fatalf("result count = %d, want 2", len(results))
	}
	if results[0].ID != "sources" || results[1].ID != "emby" {
		t.Fatalf("unexpected order: %#v", results)
	}
}
