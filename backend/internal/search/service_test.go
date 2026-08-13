package search

import (
	"context"
	"errors"
	"testing"
)

type sourceStub struct {
	id         string
	label      string
	candidates []Candidate
	err        error
}

func (stub sourceStub) ID() string    { return stub.id }
func (stub sourceStub) Label() string { return stub.label }
func (stub sourceStub) Search(context.Context, string) ([]Candidate, error) {
	return stub.candidates, stub.err
}

func TestServiceReturnsPartialResultsAndUpdatesHealth(t *testing.T) {
	service := NewService(
		sourceStub{id: "frame", label: "帧影", candidates: []Candidate{{
			ID: "release-1", Title: "范海辛", MediaType: "movie",
			Release: ReleaseFacts{Resolution: "2160p", VideoCodec: "HEVC"},
		}}},
		sourceStub{id: "gather", label: "聚影", err: errors.New("offline")},
	)

	response := service.Search(context.Background(), "范海辛")
	if !response.Partial || len(response.Results) != 1 || len(response.SourceErrors) != 1 {
		t.Fatalf("unexpected response: %#v", response)
	}
	if response.Results[0].ID != "frame:release-1" || response.Results[0].Source != "帧影" {
		t.Fatalf("unexpected candidate: %#v", response.Results[0])
	}

	health := service.Check(context.Background())
	if health.Status != "degraded" {
		t.Fatalf("health status = %q, want degraded", health.Status)
	}
}

type identityStub struct {
	identities []Identity
	err        error
}

func (stub identityStub) Resolve(context.Context, string) ([]Identity, error) {
	return stub.identities, stub.err
}

type transferSearchStub struct{ sourceStub }

func (transferSearchStub) StartTransfer(context.Context, TransferRequest) (TransferResult, error) {
	return TransferResult{}, nil
}

func (transferSearchStub) TransferStatus(context.Context, int64, string) (TransferResult, error) {
	return TransferResult{}, nil
}

func TestServiceRequiresVerifiedIdentityForTransfer(t *testing.T) {
	source := transferSearchStub{sourceStub: sourceStub{
		id: "frame", label: "帧影", candidates: []Candidate{{
			ID: "release-1", Title: "范海辛", Year: 2004, MediaType: "movie", SourceRef: "private",
			Release: ReleaseFacts{Resolution: "2160p", VideoCodec: "HEVC"},
		}},
	}}
	service := NewServiceWithIdentity(identityStub{identities: []Identity{{
		TMDBID: "7131", Title: "范海辛", OriginalTitle: "Van Helsing", Year: 2004, MediaType: "movie",
	}}}, source)

	response := service.Search(context.Background(), "范海辛")
	if len(response.Results) != 1 || response.Results[0].TMDBID != "7131" || response.Results[0].TransferState != "available" {
		t.Fatalf("unexpected candidate: %#v", response.Results)
	}
	withoutIdentity := NewService(source).Search(context.Background(), "范海辛")
	if withoutIdentity.Results[0].TransferState != "identity_required" {
		t.Fatalf("unverified state = %q", withoutIdentity.Results[0].TransferState)
	}
}

func TestServiceMatchesIdentityInsideReleaseTitle(t *testing.T) {
	source := transferSearchStub{sourceStub: sourceStub{
		id: "mikan", label: "蜜柑", candidates: []Candidate{{
			ID: "release-1", Title: "[北宇治字幕组] 葬送的芙莉莲 / Sousou no Frieren - 38 [1080p HEVC]", MediaType: "series", SourceID: "mikan", SourceRef: "private",
			Release: ReleaseFacts{Resolution: "1080p", VideoCodec: "HEVC"},
		}},
	}}
	service := NewServiceWithIdentity(identityStub{identities: []Identity{{
		TMDBID: "209867", Title: "葬送的芙莉莲", OriginalTitle: "Sousou no Frieren", Year: 2023, MediaType: "series",
	}}}, source)

	response := service.Search(context.Background(), "葬送的芙莉莲")
	if response.Results[0].TMDBID != "209867" || response.Results[0].TransferState != "available" {
		t.Fatalf("candidate = %#v", response.Results[0])
	}
}

func TestTitleContainsIdentityUsesLatinWordBoundariesAndRejectsShortTitles(t *testing.T) {
	for _, test := range []struct {
		candidate string
		identity  string
		want      bool
	}{
		{"[Group] Sousou no Frieren S02 [1080p]", "Sousou no Frieren", true},
		{"[Group] The Office US S01", "The Office", true},
		{"[Group] OfficeSpace 1999", "Office", false},
		{"[Group] Spirited Away [1080p]", "IT", false},
		{"[Group] What's Up [1080p]", "Up", false},
		{"[字幕组] 你好 第二季", "你好", true},
		{"[字幕组] 流浪地球2", "流浪地球", false},
		{"[字幕组] 英雄联盟", "英雄", false},
	} {
		if got := titleContainsIdentity(test.candidate, test.identity); got != test.want {
			t.Fatalf("titleContainsIdentity(%q, %q) = %v, want %v", test.candidate, test.identity, got, test.want)
		}
	}
}

func TestServiceDoesNotUseReleaseTitleContainmentForContractSources(t *testing.T) {
	source := transferSearchStub{sourceStub: sourceStub{
		id: "framehdr", label: "帧影", candidates: []Candidate{{
			ID: "release-1", Title: "[Group] Van Helsing [2160p]", MediaType: "movie", SourceRef: "private",
		}},
	}}
	service := NewServiceWithIdentity(identityStub{identities: []Identity{{
		TMDBID: "7131", Title: "范海辛", OriginalTitle: "Van Helsing", Year: 2004, MediaType: "movie",
	}}}, source)
	response := service.Search(context.Background(), "范海辛")
	if response.Results[0].TransferState != "identity_required" || response.Results[0].TMDBID != "" {
		t.Fatalf("candidate = %#v", response.Results[0])
	}
}

func TestServiceDoesNotGuessAmbiguousIdentity(t *testing.T) {
	source := transferSearchStub{sourceStub: sourceStub{
		id: "frame", label: "帧影", candidates: []Candidate{{
			ID: "release-1", Title: "同名电影", MediaType: "movie", SourceRef: "private",
			Release: ReleaseFacts{Resolution: "1080p", VideoCodec: "H.264"},
		}},
	}}
	service := NewServiceWithIdentity(identityStub{identities: []Identity{
		{TMDBID: "1", Title: "同名电影", Year: 2000, MediaType: "movie"},
		{TMDBID: "2", Title: "同名电影", Year: 2020, MediaType: "movie"},
	}}, source)
	response := service.Search(context.Background(), "同名电影")
	if response.Results[0].TransferState != "identity_required" || response.Results[0].TMDBID != "" {
		t.Fatalf("ambiguous candidate = %#v", response.Results[0])
	}
}
