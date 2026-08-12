package search

import (
	"context"
	"strings"
)

type fixtureSource struct {
	id        string
	label     string
	candidate Candidate
}

func FixtureSources() []Source {
	const poster = "https://image.tmdb.org/t/p/w500/gsFun8nATm52aGHeT8ueAel98nE.jpg"
	return []Source{
		fixtureSource{
			id: "framehdr", label: "帧影",
			candidate: Candidate{
				ID: "fixture-framehdr-remux", Year: 2004, MediaType: "movie", TMDBID: "7131", PosterURL: poster, IdentityVerified: true,
				Release: ReleaseFacts{Resolution: "2160p", VideoCodec: "HEVC", DynamicRange: "HDR10", Audio: "DTS:X 7.1", SizeBytes: 64_424_509_440},
			},
		},
		fixtureSource{
			id: "juying", label: "聚影",
			candidate: Candidate{
				ID: "fixture-gather-webdl", Year: 2004, MediaType: "movie", TMDBID: "7131", PosterURL: poster, IdentityVerified: true,
				Release: ReleaseFacts{Resolution: "1080p", VideoCodec: "H.264", DynamicRange: "SDR", Audio: "AAC 5.1", SizeBytes: 8_589_934_592},
			},
		},
	}
}

func (s fixtureSource) ID() string {
	return s.id
}

func (s fixtureSource) Label() string {
	return s.label
}

func (s fixtureSource) Search(_ context.Context, query string) ([]Candidate, error) {
	candidate := s.candidate
	candidate.Title = query
	if strings.EqualFold(query, "van helsing") || strings.Contains(query, "范海辛") {
		candidate.Title = "范海辛"
	}
	return []Candidate{candidate}, nil
}
