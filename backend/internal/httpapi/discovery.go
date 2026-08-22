package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"media-hub/backend/internal/tmdb"
)

type discoveryResponse struct {
	Items []tmdb.DiscoveryItem `json:"items"`
}

type discoveryGenresResponse struct {
	Genres []tmdb.Genre `json:"genres"`
}

func (h *handler) getTrending(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Discovery == nil {
		h.writeDiscovery(w, nil, tmdb.ErrNotConfigured)
		return
	}
	mediaType := strings.TrimSpace(r.URL.Query().Get("mediaType"))
	if mediaType == "" {
		mediaType = "all"
	}
	if mediaType != "all" && mediaType != "movie" && mediaType != "series" {
		writeProblem(w, problem{Type: "https://media-hub.local/problems/invalid-media-type", Title: "媒体类型无效", Status: http.StatusBadRequest, Code: "invalid_media_type"})
		return
	}
	items, err := h.dependencies.Discovery.Trending(r.Context(), mediaType, discoveryLimit(r))
	h.writeDiscovery(w, items, err)
}

func (h *handler) getDiscoveryCatalog(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Discovery == nil {
		h.writeDiscovery(w, nil, tmdb.ErrNotConfigured)
		return
	}
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	mediaType := strings.TrimSpace(r.URL.Query().Get("mediaType"))
	genreID := strings.TrimSpace(r.URL.Query().Get("genreId"))
	if mediaType == "tv" {
		mediaType = "series"
	}
	if mediaType != "movie" && mediaType != "series" {
		writeProblem(w, problem{Type: "https://media-hub.local/problems/invalid-media-type", Title: "媒体类型无效", Status: http.StatusBadRequest, Code: "invalid_media_type"})
		return
	}
	if kind != "popular" && kind != "top_rated" && kind != "genre" {
		writeProblem(w, problem{Type: "https://media-hub.local/problems/invalid-discovery-kind", Title: "发现分类无效", Status: http.StatusBadRequest, Code: "invalid_discovery_kind"})
		return
	}
	if kind == "genre" && !positiveID(genreID) {
		writeProblem(w, problem{Type: "https://media-hub.local/problems/invalid-genre", Title: "类型无效", Status: http.StatusBadRequest, Code: "invalid_genre"})
		return
	}
	items, err := h.dependencies.Discovery.Catalog(r.Context(), kind, mediaType, genreID, discoveryLimit(r))
	h.writeDiscovery(w, items, err)
}

func (h *handler) getDiscoveryGenres(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Discovery == nil {
		writeProblem(w, problem{Type: "https://media-hub.local/problems/tmdb-unavailable", Title: "TMDB 尚未配置", Status: http.StatusServiceUnavailable, Code: "tmdb_unconfigured"})
		return
	}
	mediaType := strings.TrimSpace(r.URL.Query().Get("mediaType"))
	if mediaType == "tv" {
		mediaType = "series"
	}
	if mediaType == "" {
		mediaType = "movie"
	}
	if mediaType != "movie" && mediaType != "series" {
		writeProblem(w, problem{Type: "https://media-hub.local/problems/invalid-media-type", Title: "媒体类型无效", Status: http.StatusBadRequest, Code: "invalid_media_type"})
		return
	}
	genres, err := h.dependencies.Discovery.Genres(r.Context(), mediaType)
	if err != nil {
		h.writeDiscovery(w, nil, err)
		return
	}
	writeJSON(w, http.StatusOK, discoveryGenresResponse{Genres: genres})
}

func (h *handler) getRecommendations(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Discovery == nil {
		h.writeDiscovery(w, nil, tmdb.ErrNotConfigured)
		return
	}
	mediaType := strings.ToLower(strings.TrimSpace(r.PathValue("mediaType")))
	if mediaType == "tv" {
		mediaType = "series"
	}
	tmdbID := r.PathValue("tmdbId")
	if (mediaType != "movie" && mediaType != "series") || !positiveID(tmdbID) {
		writeProblem(w, problem{Type: "https://media-hub.local/problems/invalid-media-identity", Title: "TMDB 身份无效", Status: http.StatusBadRequest, Code: "invalid_media_identity"})
		return
	}
	items, err := h.dependencies.Discovery.Recommendations(r.Context(), mediaType, tmdbID, discoveryLimit(r))
	h.writeDiscovery(w, items, err)
}

func (h *handler) writeDiscovery(w http.ResponseWriter, items []tmdb.DiscoveryItem, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, discoveryResponse{Items: items})
		return
	}
	if errors.Is(err, tmdb.ErrNotConfigured) {
		writeProblem(w, problem{Type: "https://media-hub.local/problems/tmdb-unavailable", Title: "TMDB 尚未配置", Status: http.StatusServiceUnavailable, Code: "tmdb_unconfigured"})
		return
	}
	writeProblem(w, problem{Type: "https://media-hub.local/problems/tmdb-unavailable", Title: "TMDB 发现服务暂时不可用", Status: http.StatusBadGateway, Code: "tmdb_unavailable"})
}

func discoveryLimit(r *http.Request) int {
	limit := 20
	if value, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && value >= 1 && value <= 20 {
		limit = value
	}
	return limit
}

func positiveID(value string) bool {
	parsed, err := strconv.ParseInt(value, 10, 64)
	return err == nil && parsed > 0
}
