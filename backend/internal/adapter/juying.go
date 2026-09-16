package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"media-hub/backend/internal/mediaidentity"
	"media-hub/backend/internal/search"
)

const (
	defaultJuyingURL   = "https://www.jying.top"
	juyingMaxPageBytes = 4 << 20
	juyingMaxMovies    = 10
	juyingMaxResults   = 100
)

type Juying struct {
	baseURL  string
	authMode string
	account  string
	secret   string
	client   *http.Client
	receiver ShareReceiver

	sessionMu sync.Mutex
	userToken string
}

type juyingMovie struct {
	ID          json.RawMessage `json:"id"`
	Title       string          `json:"title"`
	Cover       string          `json:"cover"`
	ReleaseYear json.RawMessage `json:"release_year"`
	MovieType   string          `json:"movie_type"`
	TMDBID      json.RawMessage `json:"tmdb_id"`
}

type juyingResource struct {
	ID                  json.RawMessage `json:"id"`
	ResourceType        string          `json:"resource_type"`
	ShareLink           string          `json:"share_link"`
	Description         string          `json:"description"`
	ResourceDescription string          `json:"resource_description"`
	Title               string          `json:"title"`
	FileSize            json.RawMessage `json:"file_size"`
	ExtractionCode      string          `json:"extraction_code"`
	LinkExposed         bool            `json:"link_exposed"`
	AccessTicket        string          `json:"access_ticket"`
	AccessEndpoint      string          `json:"access_endpoint"`
	AccessMode          string          `json:"access_mode"`
}

type juyingMoviesResponse struct {
	Status  string        `json:"status"`
	Message string        `json:"message"`
	Results []juyingMovie `json:"results"`
}

type juyingResourcesResponse struct {
	Status    string           `json:"status"`
	Message   string           `json:"message"`
	Title     string           `json:"title"`
	HasMore   bool             `json:"has_more"`
	Resources []juyingResource `json:"resources"`
}

type juyingReference struct {
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	MovieID     string `json:"movieId,omitempty"`
	ResourceID  string `json:"resourceId,omitempty"`
	ShareCode   string `json:"shareCode,omitempty"`
	ReceiveCode string `json:"receiveCode,omitempty"`
	Magnet      string `json:"magnet,omitempty"`
}

func NewJuying(baseURL, appID, appKey string, timeout time.Duration, offline Offline, receiver ShareReceiver, proxyURL *url.URL) *Juying {
	return NewJuyingWithAuthMode(baseURL, "developer", appID, appKey, timeout, offline, receiver, proxyURL)
}

func NewJuyingWithAuthMode(baseURL, authMode, account, secret string, timeout time.Duration, _ Offline, receiver ShareReceiver, proxyURL *url.URL) *Juying {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultJuyingURL
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	jar, _ := cookiejar.New(nil)
	mode := strings.ToLower(strings.TrimSpace(authMode))
	if mode == "" {
		mode = "developer"
	}
	return &Juying{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), authMode: mode,
		account: strings.TrimSpace(account), secret: strings.TrimSpace(secret), receiver: receiver,
		client: &http.Client{
			Timeout: timeout, Transport: transport, Jar: jar,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (s *Juying) ID() string    { return "juying" }
func (s *Juying) Label() string { return "聚影" }

func (s *Juying) Search(ctx context.Context, queryText string) ([]search.Candidate, error) {
	if s.authMode == "web" {
		return s.searchWeb(ctx, queryText)
	}
	return s.searchDeveloper(ctx, queryText)
}

func (s *Juying) searchDeveloper(ctx context.Context, queryText string) ([]search.Candidate, error) {
	queryText = strings.TrimSpace(queryText)
	if queryText == "" {
		return nil, search.Failure{Code: "invalid_query", Message: "搜索词不能为空", Retryable: false}
	}
	moviesURL := s.baseURL + "/api/dev/movies/?q=" + url.QueryEscape(queryText) + "&page=1&page_size=50"
	var response juyingMoviesResponse
	if err := s.getJSON(ctx, moviesURL, &response); err != nil {
		return nil, err
	}
	if response.Status != "success" {
		return nil, search.Failure{Code: "source_unavailable", Message: "聚影搜索失败", Retryable: false}
	}
	movies := response.Results
	if len(movies) > juyingMaxMovies {
		movies = movies[:juyingMaxMovies]
	}
	type result struct {
		index int
		rows  []search.Candidate
	}
	results := make(chan result, len(movies))
	var wg sync.WaitGroup
	for index, movie := range movies {
		wg.Add(1)
		go func(index int, movie juyingMovie) {
			defer wg.Done()
			rows, _ := s.movieCandidates(ctx, movie)
			results <- result{index: index, rows: rows}
		}(index, movie)
	}
	wg.Wait()
	close(results)
	ordered := make([][]search.Candidate, len(movies))
	for item := range results {
		ordered[item.index] = item.rows
	}
	merged := make([]search.Candidate, 0)
	for _, rows := range ordered {
		for _, row := range rows {
			if len(merged) >= juyingMaxResults {
				return merged, nil
			}
			merged = append(merged, row)
		}
	}
	return merged, nil
}

func (s *Juying) movieCandidates(ctx context.Context, movie juyingMovie) ([]search.Candidate, error) {
	movieID := rawJSONText(movie.ID)
	if movieID == "" || len(movieID) > 100 {
		return nil, nil
	}
	var response juyingResourcesResponse
	if err := s.getJSON(ctx, s.baseURL+"/api/dev/movie/"+url.PathEscape(movieID)+"/resources/", &response); err != nil {
		return nil, err
	}
	if response.Status != "success" {
		return nil, nil
	}
	title := strings.TrimSpace(movie.Title)
	if title == "" {
		title = strings.TrimSpace(response.Title)
	}
	if title == "" || len([]rune(title)) > 300 {
		return nil, nil
	}
	mediaType := juyingMediaType(movie.MovieType)
	year, _ := strconv.Atoi(rawJSONText(movie.ReleaseYear))
	tmdbID, _ := positiveJuyingID(movie.TMDBID)
	rows := make([]search.Candidate, 0, len(response.Resources))
	for _, resource := range response.Resources {
		candidate, ok := juyingCandidate(movieID, title, year, mediaType, resource)
		if ok {
			candidate.TMDBID = tmdbID
			rows = append(rows, candidate)
		}
	}
	return rows, nil
}

func juyingCandidate(movieID, title string, year int, mediaType string, resource juyingResource) (search.Candidate, bool) {
	releaseTitle := juyingResourceTitle(resource, title)
	if len([]rune(releaseTitle)) > 300 {
		return search.Candidate{}, false
	}
	var reference juyingReference
	switch strings.ToLower(strings.TrimSpace(resource.ResourceType)) {
	case "115":
		shareCode, receiveCode, err := parse115ShareURL(resource.ShareLink, resource.ExtractionCode)
		if err != nil {
			return search.Candidate{}, false
		}
		reference = juyingReference{Kind: "share", Title: releaseTitle, ShareCode: shareCode, ReceiveCode: receiveCode}
	case "magnet", "magnetlink":
		magnet, err := validateMagnetURI(resource.ShareLink)
		if err != nil {
			return search.Candidate{}, false
		}
		reference = juyingReference{Kind: "magnet", Title: releaseTitle, Magnet: magnet}
	default:
		return search.Candidate{}, false
	}
	encoded, err := json.Marshal(reference)
	if err != nil {
		return search.Candidate{}, false
	}
	sourceRef := string(encoded)
	transferState := "available"
	if reference.Kind == "magnet" {
		sourceRef = ""
		transferState = "unavailable"
	}
	mediaType, season, episodeStart, episodeEnd := mediaidentity.ApplyReleaseIdentity(mediaType, releaseTitle)
	resourceID := frameHDRReferenceID(movieID + "|" + rawJSONText(resource.ID) + "|" + string(encoded))
	return search.Candidate{
		ID:    "juying-" + movieID + "-" + resourceID,
		Title: title, Year: year, MediaType: mediaType,
		Season: season, EpisodeStart: episodeStart, EpisodeEnd: episodeEnd,
		SourceID: "juying", SourceRef: sourceRef, ReleaseTitle: releaseTitle, TransferState: transferState,
		Release: search.ReleaseFacts{
			Resolution:   mikanNormalizedResolution(releaseTitle),
			VideoCodec:   mikanNormalizedCodec(releaseTitle),
			DynamicRange: normalizedSidhubHDR(releaseTitle),
			SizeBytes: firstReleaseSizeBytes(
				rawJSONText(resource.FileSize), resource.Title, resource.Description, resource.ResourceDescription, releaseTitle,
			),
		},
	}, true
}

func (s *Juying) StartTransfer(ctx context.Context, input search.TransferRequest) (search.TransferResult, error) {
	if strings.TrimSpace(input.DestinationID) == "" {
		return search.TransferResult{}, search.Failure{Code: "invalid_selection", Message: "资源选择无效", Retryable: false}
	}
	var reference juyingReference
	if err := json.Unmarshal([]byte(input.Reference), &reference); err != nil || strings.TrimSpace(reference.Title) == "" {
		return search.TransferResult{}, search.Failure{Code: "invalid_selection", Message: "资源引用无效", Retryable: false}
	}
	if reference.Kind == "web" {
		resolved, err := s.resolveWebReference(ctx, reference)
		if err != nil {
			return search.TransferResult{}, err
		}
		reference = resolved
	}
	switch reference.Kind {
	case "share":
		if !frameHDRShareCode.MatchString(reference.ShareCode) || !frameHDRAccessCode.MatchString(reference.ReceiveCode) {
			return search.TransferResult{}, search.Failure{Code: "invalid_selection", Message: "资源引用无效", Retryable: false}
		}
		if s.receiver == nil {
			return search.TransferResult{}, search.Failure{Code: "source_unavailable", Message: "115 分享接收器不可用", Retryable: false}
		}
		destinationID, storageTitle, err := ensureTransferDestination(ctx, asFolderEnsurer(s.receiver), input.DestinationID, input.Title, reference.Title)
		if err != nil {
			return search.TransferResult{}, transferFolderFailure(err)
		}
		if inspector, ok := s.receiver.(ShareInspector); ok {
			videoNames, _, err := inspector.InspectShare(ctx, reference.ShareCode, reference.ReceiveCode)
			if err != nil {
				return search.TransferResult{}, search.Failure{Code: "source_unavailable", Message: "115 分享打不开，已跳过这条资源", Retryable: automaticWriteRetryAllowed(err)}
			}
			if mediaidentity.ShareContentConflictsWithMediaType(input.MediaType, videoNames) {
				return search.TransferResult{}, search.Failure{Code: "source_identity_mismatch", Message: "分享内容像是电视剧分集，请按剧集重新搜索", Retryable: false}
			}
		}
		if err := s.receiver.ReceiveShare(ctx, destinationID, reference.ShareCode, reference.ReceiveCode, nil); err != nil {
			var uncertain interface{ SubmissionUncertain() bool }
			if errors.As(err, &uncertain) && uncertain.SubmissionUncertain() {
				return search.TransferResult{}, search.Failure{Code: "source_submission_unknown", Message: "115 分享接收结果未知，需要人工确认", Retryable: true}
			}
			return search.TransferResult{}, search.Failure{Code: "source_unavailable", Message: "聚影资源接收到 115 失败", Retryable: automaticWriteRetryAllowed(err)}
		}
		return search.TransferResult{OperationID: input.IdempotencyKey, Status: "completed", FileID: destinationID, Path: storageTitle, IsFile: false}, nil
	case "magnet":
		if _, err := validateMagnetURI(reference.Magnet); err != nil {
			return search.TransferResult{}, search.Failure{Code: "invalid_selection", Message: "资源引用无效", Retryable: false}
		}
		return search.TransferResult{}, search.Failure{Code: "source_identity_mismatch", Message: "聚影磁力无法验证内部文件身份", Retryable: false}
	default:
		return search.TransferResult{}, search.Failure{Code: "invalid_selection", Message: "资源引用无效", Retryable: false}
	}
}

func (s *Juying) TransferStatus(_ context.Context, _ int64, operationID string) (search.TransferResult, error) {
	return search.TransferResult{}, search.Failure{Code: "invalid_source_response", Message: fmt.Sprintf("聚影转存没有可查询的异步状态: %s", operationID), Retryable: false}
}

func (s *Juying) getJSON(ctx context.Context, rawURL string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return search.Failure{Code: "source_unavailable", Message: "聚影请求失败", Retryable: true}
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "MediaHub/1.0")
	request.Header.Set("X-App-ID", s.account)
	request.Header.Set("X-App-Key", s.secret)
	response, err := s.client.Do(request)
	if err != nil {
		return search.Failure{Code: "source_unavailable", Message: "聚影暂时不可用", Retryable: true}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return search.Failure{Code: "source_unavailable", Message: fmt.Sprintf("聚影返回 HTTP %d", response.StatusCode), Retryable: response.StatusCode >= 500}
	}
	limited := io.LimitReader(response.Body, juyingMaxPageBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil || len(body) > juyingMaxPageBytes {
		return search.Failure{Code: "source_unavailable", Message: "聚影响应无效", Retryable: true}
	}
	if err := json.Unmarshal(body, target); err != nil {
		return search.Failure{Code: "source_unavailable", Message: "聚影响应无效", Retryable: true}
	}
	return nil
}

func juyingMediaType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "movie", "电影":
		return "movie"
	case "tv", "series", "电视剧", "动漫":
		return "series"
	default:
		return ""
	}
}

func rawJSONText(value json.RawMessage) string {
	if len(value) == 0 || string(value) == "null" {
		return ""
	}
	var text string
	if json.Unmarshal(value, &text) == nil {
		return strings.TrimSpace(text)
	}
	var number json.Number
	if json.Unmarshal(value, &number) == nil {
		return number.String()
	}
	return ""
}

func parse115ShareURL(raw, fallbackCode string) (string, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Port() != "" || parsed.Fragment != "" {
		return "", "", errors.New("invalid share URL")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "115.com" && host != "www.115.com" && host != "115cdn.com" && host != "www.115cdn.com" {
		return "", "", errors.New("unsupported share host")
	}
	segments := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(segments) != 2 || segments[0] != "s" {
		return "", "", errors.New("invalid share path")
	}
	shareCode, err := url.PathUnescape(segments[1])
	if err != nil || !frameHDRShareCode.MatchString(shareCode) {
		return "", "", errors.New("invalid share code")
	}
	query := parsed.Query()
	for key := range query {
		if key != "password" && key != "pwd" {
			return "", "", errors.New("unexpected share query")
		}
	}
	receiveCode := strings.TrimSpace(query.Get("password"))
	if receiveCode == "" {
		receiveCode = strings.TrimSpace(query.Get("pwd"))
	}
	if receiveCode == "" {
		receiveCode = strings.TrimSpace(fallbackCode)
	}
	if !frameHDRAccessCode.MatchString(receiveCode) {
		return "", "", errors.New("invalid receive code")
	}
	return shareCode, receiveCode, nil
}

func validateMagnetURI(raw string) (string, error) {
	magnet := strings.TrimSpace(raw)
	if magnet == "" || len(magnet) > 8192 || strings.ContainsAny(magnet, "\r\n") {
		return "", errors.New("invalid magnet URI")
	}
	parsed, err := url.Parse(magnet)
	if err != nil || !strings.EqualFold(parsed.Scheme, "magnet") || parsed.Host != "" || parsed.Path != "" || parsed.Opaque != "" || parsed.Fragment != "" {
		return "", errors.New("invalid magnet URI")
	}
	xt := parsed.Query().Get("xt")
	if !strings.HasPrefix(strings.ToLower(xt), "urn:btih:") || !sidhubBTIH.MatchString(xt[len("urn:btih:"):]) {
		return "", errors.New("invalid BTIH")
	}
	return magnet, nil
}

func juyingResourceTitle(resource juyingResource, fallback string) string {
	return strings.TrimSpace(firstNonEmptyString(resource.Title, resource.Description, resource.ResourceDescription, fallback))
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
