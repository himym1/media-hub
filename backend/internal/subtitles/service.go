package subtitles

import (
	"context"
	"errors"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"media-hub/backend/internal/assrt"
	"media-hub/backend/internal/emby"
	"media-hub/backend/internal/strm"
)

const (
	assrtIDPrefix     = "assrt:"
	assrtSearchBudget = 12 * time.Second
	attachAssrtTries  = 4
)

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
		if !fallbackToEmby(err) {
			return nil, err
		}
	}
	return s.emby.SearchRemoteSubtitles(ctx, itemID, language)
}

func fallbackToEmby(err error) bool {
	if errors.Is(err, assrt.ErrUnauthorized) || errors.Is(err, assrt.ErrUpstreamResponse) || errors.Is(err, assrt.ErrNotConfigured) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var timeout net.Error
	return errors.As(err, &timeout) && timeout.Timeout()
}

func (s *Service) Local(ctx context.Context, itemID string) (strm.Sidecar, error) {
	if s.emby == nil {
		return strm.Sidecar{}, emby.ErrNotConfigured
	}
	target, err := s.emby.SubtitleTarget(ctx, itemID)
	if err != nil {
		return strm.Sidecar{}, err
	}
	mount := ""
	if s.mount != nil {
		mount = s.mount()
	}
	mediaPath, err := strm.ResolveLibraryFile(mount, target.Path)
	if err != nil {
		return strm.Sidecar{}, strm.ErrSidecarNotFound
	}
	return strm.ReadSidecar(mediaPath, "chi")
}

func (s *Service) AttachChinese(ctx context.Context, itemID string) (string, error) {
	if s == nil || s.emby == nil {
		return AttachMissing, emby.ErrNotConfigured
	}
	if _, err := s.Local(ctx, itemID); err == nil {
		if target, targetErr := s.emby.SubtitleTarget(ctx, itemID); targetErr == nil {
			if mediaPath, pathErr := strm.ResolveLibraryFile(s.mountPath(), target.Path); pathErr == nil {
				_ = strm.PromoteExternalSidecar(mediaPath)
			}
		}
		return AttachExisted, nil
	}
	hits, err := s.Search(ctx, itemID, "chi")
	if err != nil {
		return AttachMissing, err
	}
	mediaPath := ""
	if target, targetErr := s.emby.SubtitleTarget(ctx, itemID); targetErr == nil {
		mediaPath = target.Path
	}
	ranked := rankChinese(hits, mediaPath)
	if len(ranked) == 0 {
		return AttachMissing, nil
	}
	var last error
	for index, picked := range ranked {
		if index >= attachAssrtTries {
			break
		}
		if err := s.Download(ctx, itemID, picked.ID); err != nil {
			last = err
			if !retryableSubtitleDownload(err) {
				return AttachMissing, err
			}
			continue
		}
		if target, targetErr := s.emby.SubtitleTarget(ctx, itemID); targetErr == nil {
			if mediaPath, pathErr := strm.ResolveLibraryFile(s.mountPath(), target.Path); pathErr == nil {
				_ = strm.PromoteExternalSidecar(mediaPath)
			}
		}
		return AttachAdded, nil
	}
	if last != nil {
		return AttachMissing, last
	}
	return AttachMissing, nil
}

func retryableSubtitleDownload(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, assrt.ErrUnauthorized) || errors.Is(err, assrt.ErrNotConfigured) {
		return false
	}
	return true
}

func (s *Service) mountPath() string {
	if s == nil || s.mount == nil {
		return ""
	}
	return s.mount()
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
	searchCtx, cancel := context.WithTimeout(ctx, assrtSearchBudget)
	defer cancel()
	target, err := s.emby.SubtitleTarget(searchCtx, itemID)
	if err != nil {
		return nil, err
	}
	search := assrt.SearchTarget{
		Title: target.Name, SeriesName: target.SeriesName, OriginalTitle: target.OriginalTitle,
		FileName: target.Path, Year: target.Year, Season: target.Season, Episode: target.Episode, Type: target.Type,
	}
	seen := make(map[int]struct{})
	results := make([]emby.RemoteSubtitle, 0, 16)
	scores := make(map[string]int, 16)
	for _, query := range assrt.Queries(search) {
		hits, err := s.assrt.Search(searchCtx, query.Text, query.FileName)
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
			scores[assrtIDPrefix+strconv.Itoa(hit.ID)] = assrt.ScoreHit(hit, search)
			if len(results) >= 20 {
				sortRemoteSubtitles(results, scores)
				return results, nil
			}
		}
		if len(results) > 0 {
			break
		}
	}
	sortRemoteSubtitles(results, scores)
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
	name, body, err := s.assrt.DownloadFile(ctx, subtitleID, assrt.FileHint{
		Season: target.Season, Episode: target.Episode, FileName: target.Path,
	})
	if err != nil {
		return err
	}
	if _, err := strm.WriteSidecar(mediaPath, "chi", name, body); err != nil {
		return err
	}
	return s.emby.RefreshItem(ctx, itemID)
}

func (s *Service) RemoveLocal(ctx context.Context, itemID string) error {
	if s.emby == nil {
		return nil
	}
	target, err := s.emby.SubtitleTarget(ctx, itemID)
	if err != nil {
		return nil
	}
	if !strings.EqualFold(target.Type, "Episode") && !strings.EqualFold(target.Type, "Movie") {
		return nil
	}
	mount := ""
	if s.mount != nil {
		mount = s.mount()
	}
	mediaPath, err := strm.ResolveLibraryFile(mount, target.Path)
	if err != nil {
		return nil
	}
	return strm.RemoveSidecars(mediaPath)
}

func sortRemoteSubtitles(items []emby.RemoteSubtitle, scores map[string]int) {
	sort.SliceStable(items, func(i, j int) bool {
		return scores[items[i].ID] > scores[items[j].ID]
	})
}

func chineseLanguage(language string) bool {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "", "chi", "zho", "zh", "zh-cn", "zh-hans", "zh-sg", "chinese", "zh-tw", "zh-hk", "zh-hant":
		return true
	default:
		return false
	}
}
