package search

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

var (
	operationIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,200}$`)
	tmdbIDPattern      = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)
)

type HTTPSource struct {
	id      string
	label   string
	baseURL string
	token   string
	client  *http.Client
}

type httpSourceResponse struct {
	Results []httpCandidate `json:"results"`
}

type httpCandidate struct {
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Year         int          `json:"year"`
	Season       int          `json:"season"`
	EpisodeStart int          `json:"episodeStart"`
	EpisodeEnd   int          `json:"episodeEnd"`
	MediaType    string       `json:"mediaType"`
	TMDBID       string       `json:"tmdbId"`
	PosterURL    string       `json:"posterUrl"`
	Release      ReleaseFacts `json:"release"`
	Reference    string       `json:"reference"`
}

type transferRequestPayload struct {
	Reference     string `json:"reference"`
	DestinationID string `json:"destinationId"`
}

type transferResponsePayload struct {
	OperationID string `json:"operationId"`
	Status      string `json:"status"`
	FileID      string `json:"fileId"`
	Path        string `json:"path"`
	IsFile      bool   `json:"isFile"`
}

func NewHTTPSource(id, label, baseURL, token string, timeout time.Duration) *HTTPSource {
	return &HTTPSource{
		id: id, label: label, baseURL: baseURL, token: token,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (s *HTTPSource) ID() string {
	return s.id
}

func (s *HTTPSource) Label() string {
	return s.label
}

func (s *HTTPSource) Search(ctx context.Context, queryText string) ([]Candidate, error) {
	endpoint, err := s.endpoint("search")
	if err != nil {
		return nil, invalidSourceConfig()
	}
	query := endpoint.Query()
	query.Set("query", queryText)
	query.Set("limit", "50")
	endpoint.RawQuery = query.Encode()

	request, err := s.request(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, invalidSourceConfig()
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, unavailableSource()
	}
	defer response.Body.Close()
	if failure := sourceHTTPFailure(response.StatusCode, "搜索"); failure != nil {
		return nil, *failure
	}

	var payload httpSourceResponse
	if err := decodeLimited(response.Body, 4<<20, &payload); err != nil {
		return nil, invalidSourceResponse()
	}
	candidates := make([]Candidate, 0, len(payload.Results))
	for _, item := range payload.Results {
		candidate, ok := normalizeHTTPCandidate(item)
		if !ok {
			return nil, invalidSourceResponse()
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func (s *HTTPSource) StartTransfer(ctx context.Context, input TransferRequest) (TransferResult, error) {
	endpoint, err := s.endpoint("transfer")
	if err != nil {
		return TransferResult{}, invalidSourceConfig()
	}
	body, err := json.Marshal(transferRequestPayload{
		Reference: input.Reference, DestinationID: input.DestinationID,
	})
	if err != nil {
		return TransferResult{}, invalidSourceConfig()
	}
	request, err := s.request(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return TransferResult{}, invalidSourceConfig()
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", input.IdempotencyKey)
	return s.executeTransferRequest(request)
}

func (s *HTTPSource) TransferStatus(ctx context.Context, _ int64, operationID string) (TransferResult, error) {
	if !operationIDPattern.MatchString(operationID) {
		return TransferResult{}, invalidSourceResponse()
	}
	endpoint, err := s.endpoint("transfer", operationID)
	if err != nil {
		return TransferResult{}, invalidSourceConfig()
	}
	request, err := s.request(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return TransferResult{}, invalidSourceConfig()
	}
	return s.executeTransferRequest(request)
}

func (s *HTTPSource) executeTransferRequest(request *http.Request) (TransferResult, error) {
	response, err := s.client.Do(request)
	if err != nil {
		return TransferResult{}, unavailableSource()
	}
	defer response.Body.Close()
	if failure := sourceHTTPFailure(response.StatusCode, "转存"); failure != nil {
		return TransferResult{}, *failure
	}
	var payload transferResponsePayload
	if err := decodeLimited(response.Body, 1<<20, &payload); err != nil {
		return TransferResult{}, invalidSourceResponse()
	}
	result := TransferResult{
		OperationID: strings.TrimSpace(payload.OperationID),
		Status:      strings.TrimSpace(payload.Status),
		FileID:      strings.TrimSpace(payload.FileID),
		Path:        strings.TrimSpace(payload.Path),
		IsFile:      payload.IsFile,
	}
	if !validTransferResult(result) {
		return TransferResult{}, invalidSourceResponse()
	}
	return result, nil
}

func (s *HTTPSource) request(ctx context.Context, method string, endpoint *url.URL, body []byte) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Media-Hub/search-source")
	if s.token != "" {
		request.Header.Set("Authorization", "Bearer "+s.token)
	}
	return request, nil
}

func (s *HTTPSource) endpoint(parts ...string) (*url.URL, error) {
	parsed, err := url.Parse(s.baseURL)
	if err != nil {
		return nil, err
	}
	pathParts := append([]string{"/", parsed.Path}, parts...)
	parsed.Path = path.Join(pathParts...)
	return parsed, nil
}

func normalizeHTTPCandidate(item httpCandidate) (Candidate, bool) {
	item.ID = strings.TrimSpace(item.ID)
	item.Title = strings.TrimSpace(item.Title)
	item.MediaType = strings.TrimSpace(item.MediaType)
	item.TMDBID = strings.TrimSpace(item.TMDBID)
	item.Reference = strings.TrimSpace(item.Reference)
	if item.ID == "" || len(item.ID) > 200 || item.Title == "" || len([]rune(item.Title)) > 300 {
		return Candidate{}, false
	}
	if item.MediaType != "movie" && item.MediaType != "series" {
		return Candidate{}, false
	}
	if item.Year < 0 || item.Year > 2100 || item.Season < 0 || item.Season > 100 ||
		!validEpisodeRange(item.EpisodeStart, item.EpisodeEnd) {
		return Candidate{}, false
	}
	if item.MediaType == "movie" && (item.Season != 0 || item.EpisodeStart != 0 || item.EpisodeEnd != 0) {
		return Candidate{}, false
	}
	if item.MediaType == "series" && (item.Season == 0 || item.EpisodeStart == 0 || item.EpisodeEnd == 0) {
		return Candidate{}, false
	}
	if item.TMDBID != "" && !tmdbIDPattern.MatchString(item.TMDBID) {
		return Candidate{}, false
	}
	if item.Release.Resolution == "" || item.Release.VideoCodec == "" || item.Release.SizeBytes < 0 {
		return Candidate{}, false
	}
	if item.PosterURL != "" {
		posterURL, err := url.Parse(item.PosterURL)
		if err != nil || (posterURL.Scheme != "http" && posterURL.Scheme != "https") || posterURL.Host == "" {
			return Candidate{}, false
		}
	}
	if len(item.Reference) > 4096 {
		return Candidate{}, false
	}
	return Candidate{
		ID: item.ID, Title: item.Title, Year: item.Year, Season: item.Season,
		EpisodeStart: item.EpisodeStart, EpisodeEnd: item.EpisodeEnd,
		MediaType: item.MediaType, TMDBID: item.TMDBID,
		PosterURL: item.PosterURL, Release: item.Release, SourceRef: item.Reference,
	}, true
}

func validEpisodeRange(start, end int) bool {
	if start == 0 || end == 0 {
		return start == 0 && end == 0
	}
	return start > 0 && end >= start && end <= 10000
}

func validTransferResult(result TransferResult) bool {
	if result.Status == "pending" {
		return operationIDPattern.MatchString(result.OperationID)
	}
	if result.Status != "completed" {
		return false
	}
	return result.FileID != "" && len(result.FileID) <= 200 && result.Path != "" && len(result.Path) <= 4096
}

func decodeLimited(reader io.Reader, limit int64, target any) error {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil || int64(len(data)) > limit {
		return io.ErrUnexpectedEOF
	}
	return json.Unmarshal(data, target)
}

func sourceHTTPFailure(status int, operation string) *Failure {
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		failure := Failure{Code: "source_unauthorized", Message: "资源源鉴权失败", Retryable: false}
		return &failure
	case status == http.StatusTooManyRequests:
		failure := Failure{Code: "source_rate_limited", Message: "资源源请求过于频繁", Retryable: true}
		return &failure
	default:
		failure := Failure{
			Code: "source_rejected", Message: "资源源拒绝了" + operation + "请求", Retryable: status >= 500,
		}
		return &failure
	}
}

func invalidSourceConfig() Failure {
	return Failure{Code: "source_invalid_config", Message: "资源源配置无效", Retryable: false}
}

func unavailableSource() Failure {
	return Failure{Code: "source_unavailable", Message: "资源源连接失败", Retryable: true}
}

func invalidSourceResponse() Failure {
	return Failure{Code: "source_invalid_response", Message: "资源源返回格式不兼容", Retryable: false}
}
