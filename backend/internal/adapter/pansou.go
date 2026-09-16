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
	"strings"
	"time"
	"unicode/utf8"

	"media-hub/backend/internal/mediaidentity"
	"media-hub/backend/internal/search"
)

const (
	defaultPansouURL   = "http://172.17.0.1:57081"
	pansouMaxPageBytes = 4 << 20
	pansouMaxResults   = 50
)

// Pansou 把盘搜 /api/search 里的 115 分享映射到现有转存链路。TG 频道留在盘搜服务内。
type Pansou struct {
	baseURL  string
	token    string
	client   *http.Client
	receiver ShareReceiver
}

type pansouReference struct {
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	ShareCode   string `json:"shareCode,omitempty"`
	ReceiveCode string `json:"receiveCode,omitempty"`
}

type pansouAPIResponse struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Data    pansouAPIData `json:"data"`
}

type pansouAPIData struct {
	Total        int                           `json:"total"`
	Results      []pansouSearchItem            `json:"results"`
	MergedByType map[string][]pansouMergedLink `json:"merged_by_type"`
}

type pansouSearchItem struct {
	UniqueID string       `json:"unique_id"`
	Title    string       `json:"title"`
	Content  string       `json:"content"`
	Links    []pansouLink `json:"links"`
}

type pansouLink struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Password  string `json:"password"`
	WorkTitle string `json:"work_title"`
}

type pansouMergedLink struct {
	URL      string `json:"url"`
	Password string `json:"password"`
	Note     string `json:"note"`
}

type pansouSearchRequest struct {
	Keyword    string   `json:"kw"`
	ResultType string   `json:"res"`
	CloudTypes []string `json:"cloud_types"`
}

func NewPansou(baseURL, token string, timeout time.Duration, receiver ShareReceiver) *Pansou {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultPansouURL
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	return &Pansou{
		baseURL:  strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:    strings.TrimSpace(token),
		receiver: receiver,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (s *Pansou) ID() string    { return "pansou" }
func (s *Pansou) Label() string { return "盘搜" }

func (s *Pansou) Search(ctx context.Context, queryText string) ([]search.Candidate, error) {
	queryText = strings.TrimSpace(queryText)
	if queryText == "" {
		return nil, search.Failure{Code: "invalid_query", Message: "搜索词不能为空", Retryable: false}
	}
	if s.baseURL == "" {
		return nil, search.Failure{Code: "source_invalid_config", Message: "盘搜地址未配置", Retryable: false}
	}
	body, err := json.Marshal(pansouSearchRequest{Keyword: queryText, ResultType: "results", CloudTypes: []string{"115"}})
	if err != nil {
		return nil, search.Failure{Code: "source_unavailable", Message: "盘搜请求失败", Retryable: true}
	}
	endpoint, err := url.Parse(s.baseURL + "/api/search")
	if err != nil || endpoint.Host == "" {
		return nil, search.Failure{Code: "source_invalid_config", Message: "盘搜地址无效", Retryable: false}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, search.Failure{Code: "source_unavailable", Message: "盘搜请求失败", Retryable: true}
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "MediaHub/1.0")
	if s.token != "" {
		request.Header.Set("Authorization", "Bearer "+s.token)
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, search.Failure{Code: "source_unavailable", Message: "盘搜暂时不可用", Retryable: true}
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, search.Failure{Code: "source_unauthorized", Message: "盘搜鉴权失败", Retryable: false}
	}
	if response.StatusCode != http.StatusOK {
		return nil, search.Failure{Code: "source_unavailable", Message: fmt.Sprintf("盘搜返回 HTTP %d", response.StatusCode), Retryable: response.StatusCode >= 500}
	}
	limited := io.LimitReader(response.Body, pansouMaxPageBytes+1)
	payload, err := io.ReadAll(limited)
	if err != nil || len(payload) > pansouMaxPageBytes {
		return nil, search.Failure{Code: "source_unavailable", Message: "盘搜响应无效", Retryable: true}
	}
	if !utf8.Valid(payload) {
		payload = []byte(strings.ToValidUTF8(string(payload), ""))
	}
	var parsed pansouAPIResponse
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return nil, search.Failure{Code: "source_unavailable", Message: "盘搜响应无效", Retryable: true}
	}
	if parsed.Code != 0 {
		return nil, search.Failure{Code: "source_unavailable", Message: "盘搜搜索失败", Retryable: false}
	}
	return pansouCandidates(queryText, parsed.Data), nil
}

func (s *Pansou) StartTransfer(ctx context.Context, input search.TransferRequest) (search.TransferResult, error) {
	if strings.TrimSpace(input.DestinationID) == "" {
		return search.TransferResult{}, search.Failure{Code: "invalid_selection", Message: "资源选择无效", Retryable: false}
	}
	var reference pansouReference
	if err := json.Unmarshal([]byte(input.Reference), &reference); err != nil || strings.TrimSpace(reference.Title) == "" {
		return search.TransferResult{}, search.Failure{Code: "invalid_selection", Message: "资源引用无效", Retryable: false}
	}
	if reference.Kind != "share" || !frameHDRShareCode.MatchString(reference.ShareCode) || !frameHDRAccessCode.MatchString(reference.ReceiveCode) {
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
		return search.TransferResult{}, search.Failure{Code: "source_unavailable", Message: "盘搜资源接收到 115 失败", Retryable: automaticWriteRetryAllowed(err)}
	}
	return search.TransferResult{OperationID: input.IdempotencyKey, Status: "completed", FileID: destinationID, Path: storageTitle, IsFile: false}, nil
}

func (s *Pansou) TransferStatus(_ context.Context, _ int64, operationID string) (search.TransferResult, error) {
	return search.TransferResult{}, search.Failure{Code: "invalid_source_response", Message: fmt.Sprintf("盘搜转存没有可查询的异步状态: %s", operationID), Retryable: false}
}

func pansouCandidates(queryText string, data pansouAPIData) []search.Candidate {
	seen := make(map[string]struct{})
	rows := make([]search.Candidate, 0, min(len(data.Results), pansouMaxResults))
	for _, item := range data.Results {
		for _, link := range item.Links {
			candidate, ok := pansouShareCandidate(queryText, item.Title, item.Content, link.WorkTitle, "", link.Type, link.URL, link.Password, seen)
			if !ok {
				continue
			}
			rows = append(rows, candidate)
			if len(rows) >= pansouMaxResults {
				return rows
			}
		}
	}
	if len(rows) > 0 {
		return rows
	}
	for _, link := range data.MergedByType["115"] {
		candidate, ok := pansouShareCandidate(queryText, "", "", "", link.Note, "115", link.URL, link.Password, seen)
		if !ok {
			continue
		}
		rows = append(rows, candidate)
		if len(rows) >= pansouMaxResults {
			return rows
		}
	}
	return rows
}

func pansouShareCandidate(queryText, itemTitle, content, workTitle, note, cloudType, rawURL, password string, seen map[string]struct{}) (search.Candidate, bool) {
	if strings.ToLower(strings.TrimSpace(cloudType)) != "115" {
		return search.Candidate{}, false
	}
	shareCode, receiveCode, err := Parse115ShareURL(rawURL, password)
	if err != nil {
		return search.Candidate{}, false
	}
	dedupe := shareCode + "|" + receiveCode
	if _, exists := seen[dedupe]; exists {
		return search.Candidate{}, false
	}
	releaseTitle := firstNonEmptyString(strings.TrimSpace(workTitle), strings.TrimSpace(itemTitle), strings.TrimSpace(note), strings.TrimSpace(content), queryText)
	if releaseTitle == "" || len([]rune(releaseTitle)) > 300 {
		return search.Candidate{}, false
	}
	title := queryText
	if title == "" || len([]rune(title)) > 300 {
		return search.Candidate{}, false
	}
	reference, err := json.Marshal(pansouReference{Kind: "share", Title: releaseTitle, ShareCode: shareCode, ReceiveCode: receiveCode})
	if err != nil {
		return search.Candidate{}, false
	}
	seen[dedupe] = struct{}{}
	mediaType, season, episodeStart, episodeEnd := mediaidentity.ApplyReleaseIdentity("", releaseTitle)
	year := firstPositiveYear(releaseTitle, itemTitle, note, content)
	return search.Candidate{
		ID:    "pansou-" + frameHDRReferenceID(dedupe+"|"+releaseTitle),
		Title: title, Year: year, MediaType: mediaType,
		Season: season, EpisodeStart: episodeStart, EpisodeEnd: episodeEnd,
		SourceID: "pansou", SourceRef: string(reference), ReleaseTitle: releaseTitle, TransferState: "available",
		Release: search.ReleaseFacts{
			Resolution:   mikanNormalizedResolution(releaseTitle),
			VideoCodec:   mikanNormalizedCodec(releaseTitle),
			DynamicRange: normalizedSidhubHDR(releaseTitle),
			SizeBytes:    firstReleaseSizeBytes(note, content, releaseTitle),
		},
	}, true
}

func parsePansou115ShareURL(raw, fallbackCode string) (string, string, error) {
	return Parse115ShareURL(raw, fallbackCode)
}

func firstPositiveYear(values ...string) int {
	for _, value := range values {
		if year := sidhubYearValue(value); year > 0 {
			return year
		}
	}
	return 0
}
