package httpapi

import "net/http"

func (h *handler) getStatisticsSummary(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.Statistics == nil {
		writeProblem(w, problem{Type: "https://media-hub.local/problems/statistics-unavailable", Title: "运营统计不可用", Status: http.StatusServiceUnavailable, Code: "statistics_unavailable"})
		return
	}
	principal := principalFromContext(r.Context())
	summary, err := h.dependencies.Statistics.Summary(r.Context(), principal.UserID)
	if err != nil {
		writeProblem(w, problem{Type: "https://media-hub.local/problems/internal", Title: "运营统计读取失败", Status: http.StatusInternalServerError, Code: "internal_error"})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}
