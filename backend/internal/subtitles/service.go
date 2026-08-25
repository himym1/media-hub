package subtitles

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"media-hub/backend/internal/assrt"
	"media-hub/backend/internal/emby"
	"media-hub/backend/internal/strm"
)

const assrtIDPrefix = "assrt:"

type Emby interface {
	SubtitleTarget(context.Context, string) (emby.SubtitleTarget, error)
	SearchRemoteSubtitles(context.Context, string, string) ([]emby.RemoteSubtitle, error)
	DownloadRemoteSubtitle(context.Context, string, string) error
	RefreshItem(context.Context, string) error
}

type Service struct {
	emby  Emby
	assrt *assrt.Client
	mount func() string
}

func New(embyClient Emby, assrtClient *assrt.Client, mount func() string) *Service {
	return &Service{emby: embyClient, assrt: assrtClient, mount: mount}
}

func (s *Service) Search(ctx context.Context, itemID, language string) ([]emby.RemoteSubtitle, error) {
	if s.emby == nil {
		return nil, emby.ErrNotConfigured
	}
	if chineseLanguage(language) && s.assrt != nil && s.assrt.Configured() {
		hits, err := s.searchAssrt(ctx, itemID)
		if err == nil {
			return hits, nil
		}
		if !errors.Is(err, assrt.ErrUnauthorized) && !errors.Is(err, assrt.ErrUpstreamResponse) && !errors.Is(err, assrt.ErrNotConfigured) {
			return nil, err
		}
	}
	return s.emby.SearchRemoteSubtitles(ctx, itemID, language)
}

func (s *Service) Download(ctx context.Context, itemID, subtitleID string) error {
	if strings.HasPrefix(subtitleID, assrtIDPrefix) {
		return s.downloadAssrt(ctx, itemID, strings.TrimPrefix(subtitleID, assrtIDPrefix))
	}
	if s.emby == nil {
		return emby.ErrNotConfigured
	}
	return s.emby.DownloadRemoteSubtitle(ctx, itemID, subtitleID)
}

func (s *Service) searchAssrt(ctx context.Context, itemID string) ([]emby.RemoteSubtitle, error) {
	target, err := s.emby.SubtitleTarget(ctx, itemID)
	if err != nil {
		return nil, err
	}
	search := assrt.SearchTarget{
		Title: target.Name, SeriesName: target.SeriesName, OriginalTitle: target.OriginalTitle,
		FileName: target.Path, Year: target.Year, Season: target.Season, Episode: target.Episode, Type: target.Type,
	}
	seen := make(map[int]struct{})
	results := make([]emby.RemoteSubtitle, 0, 16)
	for _, query := range assrt.Queries(search) {
		hits, err := s.assrt.Search(ctx, query.Text, query.FileName)
		if err != nil {
			return nil, err
		}
		for _, hit := range hits {
			if !assrt.Relevant(hit, search) {
				continue
			}
			if _, exists := seen[hit.ID]; exists {
				continue
			}
			seen[hit.ID] = struct{}{}
			results = append(results, emby.RemoteSubtitle{
				ID:           assrtIDPrefix + strconv.Itoa(hit.ID),
				Name:         hit.Name,
				Language:     hit.Language,
				Format:       hit.Format,
				ProviderName: "Assrt",
				Author:       hit.Site,
				Comment:      hit.Comment,
			})
			if len(results) >= 20 {
				return results, nil
			}
		}
		if len(results) > 0 {
			break
		}
	}
	return results, nil
}

func (s *Service) downloadAssrt(ctx context.Context, itemID, rawID string) error {
	if s.assrt == nil || !s.assrt.Configured() {
		return assrt.ErrNotConfigured
	}
	subtitleID, err := strconv.Atoi(strings.TrimSpace(rawID))
	if err != nil || subtitleID < 1 {
		return assrt.ErrUpstreamResponse
	}
	target, err := s.emby.SubtitleTarget(ctx, itemID)
	if err != nil {
		return err
	}
	mount := ""
	if s.mount != nil {
		mount = s.mount()
	}
	mediaPath, err := strm.ResolveLibraryFile(mount, target.Path)
	if err != nil {
		return err
	}
	name, body, err := s.assrt.DownloadFile(ctx, subtitleID)
	if err != nil {
		return err
	}
	if _, err := strm.WriteSidecar(mediaPath, "chi", name, body); err != nil {
		return err
	}
	return s.emby.RefreshItem(ctx, itemID)
}

func chineseLanguage(language string) bool {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "", "chi", "zho", "zh", "zh-cn", "zh-hans", "zh-sg", "chinese", "zh-tw", "zh-hk", "zh-hant":
		return true
	default:
		return false
	}
}
