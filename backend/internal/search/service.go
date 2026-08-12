package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"media-hub/backend/internal/integration"
)

type sourceState struct {
	attempted bool
	healthy   bool
}

type Service struct {
	sources  []Source
	identity IdentityResolver
	mutex    sync.RWMutex
	states   map[string]sourceState
}

type sourceOutcome struct {
	index      int
	candidates []Candidate
	err        error
}

func NewService(sources ...Source) *Service {
	return NewServiceWithIdentity(nil, sources...)
}

func NewServiceWithIdentity(identity IdentityResolver, sources ...Source) *Service {
	return &Service{
		sources:  append([]Source(nil), sources...),
		identity: identity,
		states:   make(map[string]sourceState, len(sources)),
	}
}

func (s *Service) Check(context.Context) integration.Health {
	health := integration.Health{ID: "sources", Label: "资源源"}
	if len(s.sources) == 0 {
		health.Status = integration.StatusUnconfigured
		health.Detail = "等待配置首批搜索适配器"
		return health
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()
	attempted := 0
	healthy := 0
	for _, source := range s.sources {
		state := s.states[source.ID()]
		if state.attempted {
			attempted++
		}
		if state.healthy {
			healthy++
		}
	}

	switch {
	case attempted == 0:
		health.Status = integration.StatusDegraded
		health.Detail = fmt.Sprintf("已配置 %d 个资源源，等待首次搜索", len(s.sources))
	case healthy == len(s.sources):
		health.Status = integration.StatusHealthy
		health.Detail = fmt.Sprintf("%d 个资源源可用", healthy)
	case healthy > 0:
		health.Status = integration.StatusDegraded
		health.Detail = fmt.Sprintf("%d/%d 个资源源可用", healthy, len(s.sources))
	default:
		health.Status = integration.StatusUnavailable
		health.Detail = "所有资源源暂时不可用"
	}
	return health
}

func (s *Service) Search(ctx context.Context, query string) Response {
	response := Response{
		Query:        query,
		Results:      []Candidate{},
		SourceErrors: []SourceError{},
	}
	if len(s.sources) == 0 {
		return response
	}

	identityOutcomes := make(chan struct {
		identities []Identity
		err        error
	}, 1)
	if s.identity != nil {
		go func() {
			identities, err := s.identity.Resolve(ctx, query)
			identityOutcomes <- struct {
				identities []Identity
				err        error
			}{identities: identities, err: err}
		}()
	}

	outcomes := make(chan sourceOutcome, len(s.sources))
	for index, source := range s.sources {
		go func() {
			candidates, err := source.Search(ctx, query)
			outcomes <- sourceOutcome{index: index, candidates: candidates, err: err}
		}()
	}

	ordered := make([]sourceOutcome, len(s.sources))
	for range s.sources {
		outcome := <-outcomes
		ordered[outcome.index] = outcome
	}

	var identities []Identity
	if s.identity != nil {
		outcome := <-identityOutcomes
		identities = outcome.identities
		if outcome.err != nil {
			response.SourceErrors = append(response.SourceErrors, SourceError{
				Source: "TMDB", Code: "identity_unavailable",
				Message: "影视身份服务暂时不可用", Retryable: true,
			})
		}
	}

	for index, outcome := range ordered {
		source := s.sources[index]
		s.setSourceState(source.ID(), outcome.err == nil)
		if outcome.err != nil {
			response.SourceErrors = append(response.SourceErrors, publicSourceError(source, outcome.err))
			continue
		}
		_, supportsTransfer := source.(TransferSource)
		for _, candidate := range outcome.candidates {
			if !candidate.IdentityVerified {
				candidate.PosterURL = ""
			}
			candidate.ID = source.ID() + ":" + candidate.ID
			candidate.SourceID = source.ID()
			candidate.Source = source.Label()
			if identity, ok := matchIdentity(candidate, identities); ok {
				candidate.TMDBID = identity.TMDBID
				candidate.IdentityVerified = true
				if candidate.Year == 0 {
					candidate.Year = identity.Year
				}
				candidate.PosterURL = identity.PosterURL
			}
			if supportsTransfer && candidate.SourceRef != "" && candidate.IdentityVerified {
				candidate.TransferState = "available"
			} else if supportsTransfer && candidate.SourceRef != "" {
				candidate.TransferState = "identity_required"
			} else {
				candidate.TransferState = "unavailable"
			}
			response.Results = append(response.Results, candidate)
		}
	}
	response.Partial = len(response.SourceErrors) > 0
	return response
}

func matchIdentity(candidate Candidate, identities []Identity) (Identity, bool) {
	var matched Identity
	matches := 0
	for _, identity := range identities {
		if candidate.TMDBID != "" && candidate.TMDBID != identity.TMDBID {
			continue
		}
		if candidate.MediaType != "" && candidate.MediaType != identity.MediaType {
			continue
		}
		if candidate.Year > 0 && identity.Year > 0 && candidate.Year != identity.Year {
			continue
		}
		candidateTitle := strings.TrimSpace(candidate.Title)
		if candidate.TMDBID == "" &&
			!strings.EqualFold(candidateTitle, strings.TrimSpace(identity.Title)) &&
			!strings.EqualFold(candidateTitle, strings.TrimSpace(identity.OriginalTitle)) {
			continue
		}
		if candidate.TMDBID != "" {
			return identity, true
		}
		matched = identity
		matches++
	}
	return matched, matches == 1
}

func (s *Service) setSourceState(sourceID string, healthy bool) {
	s.mutex.Lock()
	s.states[sourceID] = sourceState{attempted: true, healthy: healthy}
	s.mutex.Unlock()
}

func publicSourceError(source Source, err error) SourceError {
	var failure Failure
	if errors.As(err, &failure) {
		return SourceError{
			Source: source.Label(), Code: failure.Code,
			Message: failure.Message, Retryable: failure.Retryable,
		}
	}
	return SourceError{
		Source: source.Label(), Code: "source_unavailable",
		Message: "资源源暂时不可用", Retryable: true,
	}
}

func (s *Service) TransferSource(sourceID string) (TransferSource, bool) {
	switch sourceID {
	case "frame":
		sourceID = "framehdr"
	case "gather":
		sourceID = "juying"
	}
	for _, source := range s.sources {
		if source.ID() != sourceID {
			continue
		}
		transferSource, ok := source.(TransferSource)
		return transferSource, ok
	}
	return nil, false
}
