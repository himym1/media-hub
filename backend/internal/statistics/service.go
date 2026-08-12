package statistics

import (
	"context"

	"media-hub/backend/internal/store"
)

type Summary = store.OperationalStatistics

type Service struct {
	store *store.Store
}

func NewService(dataStore *store.Store) *Service {
	return &Service{store: dataStore}
}

func (s *Service) Summary(ctx context.Context, userID int64) (Summary, error) {
	return s.store.OperationalStatistics(ctx, userID)
}
