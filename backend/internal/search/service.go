package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode"

	"media-hub/backend/internal/integration"
)

type sourceState struct {
	attempted bool
	healthy   bool
}

type Service struct {
	mutex    sync.RWMutex
	sources  []Source
	identity IdentityResolver
	states   map[string]sourceState
	revision uint64
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
		revision: 1,
	}
}

func (s *Service) Configure(identity IdentityResolver, sources ...Source) {
	s.mutex.Lock()
	s.identity = identity
	s.sources = append([]Source(nil), sources...)
	s.states = make(map[string]sourceState, len(sources))
	s.revision++
	s.mutex.Unlock()
}

func (s *Service) snapshot() (IdentityResolver, []Source, map[string]sourceState, uint64) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	states := make(map[string]sourceState, len(s.states))
	for id, state := range s.states {
		states[id] = state
	}
	return s.identity, append([]Source(nil), s.sources...), states, s.revision
}

func (s *Service) Check(context.Context) integration.Health {
	health := integration.Health{ID: "sources", Label: "资源源"}
	_, sources, states, _ := s.snapshot()
	if len(sources) == 0 {
		health.Status = integration.StatusUnconfigured
		health.Detail = "等待配置首批搜索适配器"
		return health
	}
	attempted := 0
	healthy := 0
	for _, source := range sources {
		state := states[source.ID()]
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
		health.Detail = fmt.Sprintf("已配置 %d 个资源源，等待首次搜索", len(sources))
	case healthy == len(sources):
		health.Status = integration.StatusHealthy
		health.Detail = fmt.Sprintf("%d 个资源源可用", healthy)
	case healthy > 0:
		health.Status = integration.StatusDegraded
		health.Detail = fmt.Sprintf("%d/%d 个资源源可用", healthy, len(sources))
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
	identity, sources, _, revision := s.snapshot()
	if len(sources) == 0 {
		return response
	}

	identityOutcomes := make(chan struct {
		identities []Identity
		err        error
	}, 1)
	if identity != nil {
		go func() {
			identities, err := identity.Resolve(ctx, query)
			identityOutcomes <- struct {
				identities []Identity
				err        error
			}{identities: identities, err: err}
		}()
	}

	outcomes := make(chan sourceOutcome, len(sources))
	for index, source := range sources {
		go func() {
			candidates, err := source.Search(ctx, query)
			outcomes <- sourceOutcome{index: index, candidates: candidates, err: err}
		}()
	}
	ordered := make([]sourceOutcome, len(sources))
	for range sources {
		outcome := <-outcomes
		ordered[outcome.index] = outcome
	}

	var identities []Identity
	if identity != nil {
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
		source := sources[index]
		s.setSourceState(revision, source.ID(), outcome.err == nil)
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
			candidate.Revision = revision
			candidate.Source = source.Label()
			if identity, ok := matchIdentity(candidate, identities); ok {
				candidate.TMDBID = identity.TMDBID
				candidate.IdentityVerified = true
				if candidate.Year == 0 {
					candidate.Year = identity.Year
				}
				if candidate.MediaType == "" {
					candidate.MediaType = identity.MediaType
				}
				candidate.PosterURL = identity.PosterURL
			}
			if supportsTransfer && candidate.SourceRef != "" && candidate.IdentityVerified && validTransferShape(candidate) {
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

func validTransferShape(candidate Candidate) bool {
	switch candidate.MediaType {
	case "movie":
		return candidate.Season == 0 && candidate.EpisodeStart == 0 && candidate.EpisodeEnd == 0
	case "series":
		return candidate.Season > 0 && candidate.EpisodeStart > 0 && candidate.EpisodeEnd >= candidate.EpisodeStart
	default:
		return false
	}
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
		if candidate.SourceID == "juying" && (candidate.TMDBID == "" ||
			(!releaseTitleContainsIdentity(candidate.ReleaseTitle, identity.Title) &&
				!releaseTitleContainsIdentity(candidate.ReleaseTitle, identity.OriginalTitle))) {
			continue
		}
		if candidate.TMDBID == "" &&
			!candidateTitleMatchesIdentity(candidate, identity.Title) &&
			!candidateTitleMatchesIdentity(candidate, identity.OriginalTitle) {
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

func candidateTitleMatchesIdentity(candidate Candidate, identityTitle string) bool {
	if strings.EqualFold(strings.TrimSpace(candidate.Title), strings.TrimSpace(identityTitle)) {
		return true
	}
	return candidate.SourceID == "mikan" && titleContainsIdentity(candidate.Title, identityTitle)
}

func releaseTitleContainsIdentity(releaseTitle, identityTitle string) bool {
	normalize := func(value string) string {
		return strings.Join(strings.Fields(strings.Map(func(character rune) rune {
			if unicode.IsLetter(character) || unicode.IsDigit(character) {
				return character
			}
			return ' '
		}, value)), " ")
	}
	return titleContainsIdentity(normalize(releaseTitle), normalize(identityTitle))
}

func titleContainsIdentity(candidateTitle, identityTitle string) bool {
	candidateTitle = strings.TrimSpace(candidateTitle)
	identityTitle = strings.TrimSpace(identityTitle)
	if candidateTitle == "" || identityTitle == "" {
		return false
	}
	identityRunes := []rune(strings.ToLower(identityTitle))
	if len(identityRunes) < 2 || (len(identityRunes) < 3 && allASCII(identityRunes)) {
		return false
	}
	candidateRunes := []rune(strings.ToLower(candidateTitle))
	for start := 0; start+len(identityRunes) <= len(candidateRunes); start++ {
		if string(candidateRunes[start:start+len(identityRunes)]) != string(identityRunes) {
			continue
		}
		if start > 0 && (unicode.IsLetter(candidateRunes[start-1]) || unicode.IsDigit(candidateRunes[start-1])) {
			continue
		}
		end := start + len(identityRunes)
		if end < len(candidateRunes) && (unicode.IsLetter(candidateRunes[end]) || unicode.IsDigit(candidateRunes[end])) {
			continue
		}
		return true
	}
	return false
}

func allASCII(values []rune) bool {
	for _, value := range values {
		if value > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func (s *Service) setSourceState(revision uint64, sourceID string, healthy bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.revision != revision {
		return
	}
	for _, source := range s.sources {
		if source.ID() == sourceID {
			s.states[sourceID] = sourceState{attempted: true, healthy: healthy}
			return
		}
	}
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

func (s *Service) CurrentRevision() uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.revision
}

func (s *Service) TransferSource(sourceID string) (TransferSource, bool) {
	switch sourceID {
	case "frame":
		sourceID = "framehdr"
	case "gather":
		sourceID = "juying"
	}
	_, sources, _, _ := s.snapshot()
	for _, source := range sources {
		if source.ID() != sourceID {
			continue
		}
		transferSource, ok := source.(TransferSource)
		return transferSource, ok
	}
	return nil, false
}
