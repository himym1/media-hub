package httpapi

import (
	"net/http"
	"strings"

	"media-hub/backend/internal/search"
)

type publicCandidate struct {
	search.Candidate
	TransferToken string `json:"transferToken,omitempty"`
}

type publicSearchResponse struct {
	Query        string               `json:"query"`
	Partial      bool                 `json:"partial"`
	Results      []publicCandidate    `json:"results"`
	SourceErrors []search.SourceError `json:"sourceErrors"`
}

func (h *handler) search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/query-required",
			Title: "搜索词不能为空", Status: http.StatusBadRequest,
			Code: "query_required", Detail: "请输入片名、原名或 TMDB 编号。",
		})
		return
	}
	if len([]rune(query)) > 120 {
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/query-too-long",
			Title: "搜索词过长", Status: http.StatusBadRequest,
			Code: "query_too_long",
		})
		return
	}
	if h.dependencies.Search == nil {
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/search-unavailable",
			Title: "搜索服务不可用", Status: http.StatusServiceUnavailable,
			Code: "search_unavailable",
		})
		return
	}

	response := h.dependencies.Search.Search(r.Context(), query)
	results := make([]publicCandidate, 0, len(response.Results))
	for _, candidate := range response.Results {
		if candidate.TransferState != "available" {
			continue
		}
		token := ""
		if h.dependencies.Workflow != nil {
			token = h.dependencies.Workflow.SelectionToken(candidate)
		}
		results = append(results, publicCandidate{Candidate: candidate, TransferToken: token})
	}
	writeJSON(w, http.StatusOK, publicSearchResponse{
		Query: response.Query, Partial: response.Partial,
		Results: results, SourceErrors: response.SourceErrors,
	})
}
