package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"media-hub/backend/internal/search"
)

const (
	juyingWebPageSize  = 100
	juyingWebMaxPages  = 5
	juyingWebParallel  = 3
	juyingMaxOpaqueLen = 4096
)

type juyingAccessResponse struct {
	Status     string `json:"status"`
	Target     string `json:"target"`
	AccessCode string `json:"access_code"`
	AccessMode string `json:"access_mode"`
}

func (s *Juying) searchWeb(ctx context.Context, queryText string) ([]search.Candidate, error) {
	queryText = strings.TrimSpace(queryText)
	if queryText == "" {
		return nil, search.Failure{Code: "invalid_query", Message: "搜索词不能为空", Retryable: false}
	}
	var response juyingMoviesResponse
	path := "/api/app/movies/?q=" + url.QueryEscape(queryText) + "&page=1&page_size=50&count=true&exact=false"
	if err := s.webJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
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
		err   error
	}
	results := make(chan result, len(movies))
	semaphore := make(chan struct{}, juyingWebParallel)
	var wg sync.WaitGroup
	for index, movie := range movies {
		wg.Add(1)
		go func(index int, movie juyingMovie) {
			defer wg.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				results <- result{index: index, err: ctx.Err()}
				return
			}
			rows, err := s.webMovieCandidates(ctx, movie)
			results <- result{index: index, rows: rows, err: err}
		}(index, movie)
	}
	wg.Wait()
	close(results)
	ordered := make([][]search.Candidate, len(movies))
	var firstErr error
	for item := range results {
		ordered[item.index] = item.rows
		if firstErr == nil && item.err != nil {
			firstErr = item.err
		}
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
	if len(merged) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return merged, nil
}

func (s *Juying) webMovieCandidates(ctx context.Context, movie juyingMovie) ([]search.Candidate, error) {
	movieID, ok := positiveJuyingID(movie.ID)
	if !ok {
		return nil, nil
	}
	var response juyingResourcesResponse
	path := fmt.Sprintf("/api/app/movie/%s/resources/?page=1&page_size=%d", movieID, juyingWebPageSize)
	if err := s.webJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	if response.Status != "success" {
		return nil, search.Failure{Code: "source_unavailable", Message: "聚影资源列表不可用", Retryable: false}
	}
	title := strings.TrimSpace(movie.Title)
	if title == "" || len([]rune(title)) > 300 {
		return nil, nil
	}
	mediaType := juyingMediaType(movie.MovieType)
	year, _ := strconv.Atoi(rawJSONText(movie.ReleaseYear))
	tmdbID, _ := positiveJuyingID(movie.TMDBID)
	rows := make([]search.Candidate, 0, len(response.Resources))
	for _, resource := range response.Resources {
		candidate, ok := juyingWebCandidate(movieID, title, year, mediaType, resource)
		if ok {
			candidate.TMDBID = tmdbID
			rows = append(rows, candidate)
		}
	}
	return rows, nil
}

func juyingWebCandidate(movieID, title string, year int, mediaType string, resource juyingResource) (search.Candidate, bool) {
	resourceID, ok := positiveJuyingID(resource.ID)
	if !ok || !validJuyingWebResource(resourceID, resource) {
		return search.Candidate{}, false
	}
	releaseTitle := juyingResourceTitle(resource, title)
	if releaseTitle == "" || len([]rune(releaseTitle)) > 300 {
		return search.Candidate{}, false
	}
	reference := juyingReference{Kind: "web", Title: releaseTitle, MovieID: movieID, ResourceID: resourceID}
	encoded, err := json.Marshal(reference)
	if err != nil {
		return search.Candidate{}, false
	}
	sourceRef := string(encoded)
	transferState := "available"
	if strings.ToLower(strings.TrimSpace(resource.ResourceType)) != "115" {
		sourceRef = ""
		transferState = "unavailable"
	}
	season, episodeStart, episodeEnd := sidhubEpisodeRange(releaseTitle)
	if mediaType != "series" {
		season, episodeStart, episodeEnd = 0, 0, 0
	}
	resourceIDHash := frameHDRReferenceID(movieID + "|" + resourceID + "|" + releaseTitle)
	return search.Candidate{
		ID:    "juying-" + movieID + "-" + resourceIDHash,
		Title: title, Year: year, MediaType: mediaType,
		Season: season, EpisodeStart: episodeStart, EpisodeEnd: episodeEnd,
		SourceID: "juying", SourceRef: sourceRef, ReleaseTitle: releaseTitle, TransferState: transferState,
		Release: search.ReleaseFacts{
			Resolution:   firstNonEmptyString(strings.TrimSpace(resourceDescriptionResolution(resource)), mikanNormalizedResolution(releaseTitle)),
			VideoCodec:   mikanNormalizedCodec(releaseTitle),
			DynamicRange: normalizedSidhubHDR(releaseTitle),
			SizeBytes: firstReleaseSizeBytes(
				rawJSONText(resource.FileSize), resource.Title, resource.Description, resource.ResourceDescription, releaseTitle,
			),
		},
	}, true
}

func resourceDescriptionResolution(resource juyingResource) string {
	return mikanNormalizedResolution(firstNonEmptyString(resource.ResourceDescription, resource.Description, resource.Title))
}

func validJuyingWebResource(resourceID string, resource juyingResource) bool {
	kind := strings.ToLower(strings.TrimSpace(resource.ResourceType))
	if kind != "115" && kind != "magnet" && kind != "magnetlink" {
		return false
	}
	expectedEndpoint := fmt.Sprintf("/api/app/resource/%s/access/", resourceID)
	return resource.LinkExposed && validJuyingOpaque(resource.AccessTicket) && strings.TrimSpace(resource.AccessEndpoint) == expectedEndpoint
}

func (s *Juying) resolveWebReference(ctx context.Context, reference juyingReference) (juyingReference, error) {
	if s.authMode != "web" || len([]rune(reference.Title)) > 300 {
		return juyingReference{}, search.Failure{Code: "invalid_selection", Message: "资源引用无效", Retryable: false}
	}
	movieID, ok := positiveJuyingIDText(reference.MovieID)
	if !ok {
		return juyingReference{}, search.Failure{Code: "invalid_selection", Message: "资源引用无效", Retryable: false}
	}
	resourceID, ok := positiveJuyingIDText(reference.ResourceID)
	if !ok {
		return juyingReference{}, search.Failure{Code: "invalid_selection", Message: "资源引用无效", Retryable: false}
	}
	resource, err := s.webResource(ctx, movieID, resourceID)
	if err != nil {
		return juyingReference{}, err
	}
	if juyingResourceTitle(resource, reference.Title) != strings.TrimSpace(reference.Title) {
		return juyingReference{}, search.Failure{Code: "source_identity_mismatch", Message: "聚影资源身份已变化，请重新搜索", Retryable: false}
	}
	var response juyingAccessResponse
	path := fmt.Sprintf("/api/app/resource/%s/access/", resourceID)
	if err := s.webJSON(ctx, http.MethodPost, path, map[string]string{"access_ticket": resource.AccessTicket}, &response); err != nil {
		return juyingReference{}, juyingAccessFailure(err)
	}
	if response.Status != "" && response.Status != "success" {
		return juyingReference{}, search.Failure{Code: "source_unavailable", Message: "聚影资源访问失败", Retryable: false}
	}
	target := strings.TrimSpace(response.Target)
	if target == "" || len(target) > juyingMaxOpaqueLen {
		return juyingReference{}, search.Failure{Code: "source_unavailable", Message: "聚影资源链接无效", Retryable: false}
	}
	switch strings.ToLower(strings.TrimSpace(resource.ResourceType)) {
	case "115":
		shareCode, receiveCode, err := parse115ShareURL(target, response.AccessCode)
		if err != nil {
			return juyingReference{}, search.Failure{Code: "source_unavailable", Message: "聚影返回的 115 分享无效", Retryable: false}
		}
		return juyingReference{Kind: "share", Title: reference.Title, ShareCode: shareCode, ReceiveCode: receiveCode}, nil
	case "magnet", "magnetlink":
		magnet, err := validateMagnetURI(target)
		if err != nil {
			return juyingReference{}, search.Failure{Code: "source_unavailable", Message: "聚影返回的磁力链接无效", Retryable: false}
		}
		return juyingReference{Kind: "magnet", Title: reference.Title, Magnet: magnet}, nil
	default:
		return juyingReference{}, search.Failure{Code: "source_unavailable", Message: "聚影资源类型不受支持", Retryable: false}
	}
}

func juyingAccessFailure(err error) error {
	var failure search.Failure
	if errors.As(err, &failure) && failure.Code == "source_unavailable" && failure.Retryable {
		return search.Failure{Code: "source_access_unknown", Message: "聚影资源访问结果未知，需要确认后重试", Retryable: true}
	}
	return err
}

func (s *Juying) webResource(ctx context.Context, movieID, resourceID string) (juyingResource, error) {
	for page := 1; page <= juyingWebMaxPages; page++ {
		var response juyingResourcesResponse
		path := fmt.Sprintf("/api/app/movie/%s/resources/?page=%d&page_size=%d", movieID, page, juyingWebPageSize)
		if err := s.webJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
			return juyingResource{}, err
		}
		if response.Status != "success" {
			return juyingResource{}, search.Failure{Code: "source_unavailable", Message: "聚影资源列表不可用", Retryable: false}
		}
		for _, resource := range response.Resources {
			id, ok := positiveJuyingID(resource.ID)
			if ok && id == resourceID {
				if !validJuyingWebResource(id, resource) {
					return juyingResource{}, search.Failure{Code: "source_unavailable", Message: "聚影资源暂不可访问", Retryable: false}
				}
				return resource, nil
			}
		}
		if !response.HasMore {
			break
		}
	}
	return juyingResource{}, search.Failure{Code: "source_unavailable", Message: "聚影资源已失效", Retryable: false}
}

func (s *Juying) webJSON(ctx context.Context, method, path string, payload, target any) error {
	return s.webJSONAttempt(ctx, method, path, payload, target, true)
}

func (s *Juying) webJSONAttempt(ctx context.Context, method, path string, payload, target any, retryAuth bool) error {
	token, err := s.webSessionToken(ctx)
	if err != nil {
		return err
	}
	var body []byte
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return search.Failure{Code: "source_unavailable", Message: "聚影请求无效", Retryable: false}
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return search.Failure{Code: "source_unavailable", Message: "聚影请求失败", Retryable: true}
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "MediaHub/1.0")
	request.Header.Set("X-Requested-With", "XMLHttpRequest")
	request.Header.Set("X-App-User-Token", token)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
		if csrf := s.webCSRFToken(); csrf != "" {
			request.Header.Set("X-CSRFToken", csrf)
		}
	}
	response, err := s.client.Do(request)
	if err != nil {
		return search.Failure{Code: "source_unavailable", Message: "聚影暂时不可用", Retryable: true}
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized && retryAuth {
		s.invalidateWebSessionToken(token)
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, juyingMaxPageBytes+1))
		return s.webJSONAttempt(ctx, method, path, payload, target, false)
	}
	if response.StatusCode != http.StatusOK {
		return juyingWebHTTPFailure(response.StatusCode)
	}
	if refreshed := strings.TrimSpace(response.Header.Get("X-Refreshed-Token")); refreshed != "" {
		if !validJuyingOpaque(refreshed) {
			return search.Failure{Code: "source_unavailable", Message: "聚影会话响应无效", Retryable: false}
		}
		s.setWebSessionToken(refreshed)
	}
	limited := io.LimitReader(response.Body, juyingMaxPageBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil || len(data) > juyingMaxPageBytes {
		return search.Failure{Code: "source_unavailable", Message: "聚影响应无效", Retryable: true}
	}
	if err := json.Unmarshal(data, target); err != nil {
		return search.Failure{Code: "source_unavailable", Message: "聚影响应无效", Retryable: true}
	}
	return nil
}

func (s *Juying) webSessionToken(ctx context.Context) (string, error) {
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	if s.userToken != "" {
		return s.userToken, nil
	}
	token, err := s.loginWebLocked(ctx)
	if err != nil {
		return "", err
	}
	s.userToken = token
	return token, nil
}

func (s *Juying) loginWebLocked(ctx context.Context) (string, error) {
	csrfRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/api/csrf/", nil)
	if err != nil {
		return "", search.Failure{Code: "source_unavailable", Message: "聚影登录失败", Retryable: true}
	}
	csrfRequest.Header.Set("Accept", "application/json")
	csrfRequest.Header.Set("User-Agent", "MediaHub/1.0")
	csrfRequest.Header.Set("X-Requested-With", "XMLHttpRequest")
	csrfResponse, err := s.client.Do(csrfRequest)
	if err != nil {
		return "", search.Failure{Code: "source_unavailable", Message: "聚影登录失败", Retryable: true}
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(csrfResponse.Body, juyingMaxPageBytes+1))
	csrfResponse.Body.Close()
	if csrfResponse.StatusCode != http.StatusOK {
		return "", juyingWebHTTPFailure(csrfResponse.StatusCode)
	}
	csrf := s.webCSRFToken()
	if csrf == "" {
		return "", search.Failure{Code: "source_unavailable", Message: "聚影登录校验失败", Retryable: true}
	}
	payload, _ := json.Marshal(map[string]string{"username": s.account, "password": s.secret})
	loginRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/api/app/login/", bytes.NewReader(payload))
	if err != nil {
		return "", search.Failure{Code: "source_unavailable", Message: "聚影登录失败", Retryable: true}
	}
	loginRequest.Header.Set("Accept", "application/json")
	loginRequest.Header.Set("Content-Type", "application/json")
	loginRequest.Header.Set("User-Agent", "MediaHub/1.0")
	loginRequest.Header.Set("X-Requested-With", "XMLHttpRequest")
	loginRequest.Header.Set("X-CSRFToken", csrf)
	loginResponse, err := s.client.Do(loginRequest)
	if err != nil {
		return "", search.Failure{Code: "source_unavailable", Message: "聚影登录失败", Retryable: true}
	}
	defer loginResponse.Body.Close()
	if loginResponse.StatusCode != http.StatusOK {
		failure := juyingWebHTTPFailure(loginResponse.StatusCode)
		if loginResponse.StatusCode == http.StatusBadRequest || loginResponse.StatusCode == http.StatusUnauthorized || loginResponse.StatusCode == http.StatusForbidden {
			failure = search.Failure{Code: "source_unauthorized", Message: "聚影用户名或密码无效", Retryable: false}
		}
		return "", failure
	}
	limited := io.LimitReader(loginResponse.Body, juyingMaxPageBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil || len(data) > juyingMaxPageBytes {
		return "", search.Failure{Code: "source_unavailable", Message: "聚影登录响应无效", Retryable: true}
	}
	token := juyingWebLoginToken(data, loginResponse.Header)
	if token == "" {
		return "", search.Failure{Code: "source_unauthorized", Message: "聚影登录失败", Retryable: false}
	}
	return token, nil
}

func juyingWebLoginToken(data []byte, header http.Header) string {
	var payload map[string]json.RawMessage
	if json.Unmarshal(data, &payload) == nil {
		for _, key := range []string{"token", "user_token", "access_token"} {
			if token := rawJSONText(payload[key]); validJuyingOpaque(token) {
				return token
			}
		}
		if nested := payload["data"]; len(nested) > 0 {
			var inner map[string]json.RawMessage
			if json.Unmarshal(nested, &inner) == nil {
				for _, key := range []string{"token", "user_token", "access_token"} {
					if token := rawJSONText(inner[key]); validJuyingOpaque(token) {
						return token
					}
				}
			}
		}
	}
	if header != nil {
		for _, key := range []string{"X-App-User-Token", "X-Refreshed-Token"} {
			if token := strings.TrimSpace(header.Get(key)); validJuyingOpaque(token) {
				return token
			}
		}
	}
	return ""
}

func (s *Juying) webCSRFToken() string {
	parsed, err := url.Parse(s.baseURL)
	if err != nil || s.client.Jar == nil {
		return ""
	}
	for _, cookie := range s.client.Jar.Cookies(parsed) {
		if cookie.Name == "csrftoken" && validJuyingOpaque(cookie.Value) {
			return cookie.Value
		}
	}
	return ""
}

func (s *Juying) setWebSessionToken(token string) {
	s.sessionMu.Lock()
	s.userToken = token
	s.sessionMu.Unlock()
}

func (s *Juying) invalidateWebSessionToken(previous string) {
	s.sessionMu.Lock()
	if s.userToken == previous {
		s.userToken = ""
	}
	s.sessionMu.Unlock()
}

func juyingWebHTTPFailure(status int) search.Failure {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return search.Failure{Code: "source_unauthorized", Message: "聚影登录已失效", Retryable: false}
	case status == http.StatusTooManyRequests:
		return search.Failure{Code: "source_rate_limited", Message: "聚影请求过于频繁", Retryable: true}
	case status >= 500:
		return search.Failure{Code: "source_unavailable", Message: fmt.Sprintf("聚影返回 HTTP %d", status), Retryable: true}
	default:
		return search.Failure{Code: "source_unavailable", Message: fmt.Sprintf("聚影返回 HTTP %d", status), Retryable: false}
	}
}

func positiveJuyingID(value json.RawMessage) (string, bool) {
	return positiveJuyingIDText(rawJSONText(value))
}

func positiveJuyingIDText(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 20 {
		return "", false
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		return "", false
	}
	return strconv.FormatUint(parsed, 10), true
}

func validJuyingOpaque(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > juyingMaxOpaqueLen {
		return false
	}
	for _, character := range value {
		if character <= 0x20 || character > 0x7e {
			return false
		}
	}
	return true
}
