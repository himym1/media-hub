package integration

import (
	"context"
	"sync"
)

const (
	StatusHealthy      = "healthy"
	StatusDegraded     = "degraded"
	StatusUnavailable  = "unavailable"
	StatusUnconfigured = "unconfigured"
)

type Health struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type Checker interface {
	Check(context.Context) Health
}

type OverviewService struct {
	checkers []Checker
}

func NewOverviewService(checkers ...Checker) *OverviewService {
	return &OverviewService{checkers: append([]Checker(nil), checkers...)}
}

func (s *OverviewService) Overview(ctx context.Context) []Health {
	results := make([]Health, len(s.checkers))
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(s.checkers))

	for index, checker := range s.checkers {
		go func() {
			defer waitGroup.Done()
			results[index] = checker.Check(ctx)
		}()
	}

	waitGroup.Wait()
	return results
}
