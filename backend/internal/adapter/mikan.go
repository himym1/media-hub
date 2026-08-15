package adapter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"media-hub/backend/internal/search"
)

const defaultMikanURL = "https://mikanani.me"

var (
	mikanResolution = regexp.MustCompile(`(?i)\b(2160p|1080p|720p|480p|4k)\b`)
	mikanCodec      = regexp.MustCompile(`(?i)\b(hevc|x265|h[. ]?265|avc|x264|h[. ]?264|av1)\b`)
	mikanEpisode    = regexp.MustCompile(`(?i)(?:S(\d{1,2})E(\d{1,3})|[\[\s-](\d{1,3})(?:v\d+)?[\]]?(?:\s|$))`)
	mikanMovieMark  = regexp.MustCompile(`(?i:剧场版|电影|movie)`)
)

type Mikan struct {
	baseURL string
	token   string
	client  *http.Client
	offline Offline
}

type mikanReference struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type rssFeed struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title     string `xml:"title"`
	Link      string `xml:"link"`
	Enclosure struct {
		URL    string `xml:"url,attr"`
		Length string `xml:"length,attr"`
	} `xml:"enclosure"`
}

func NewMikan(baseURL, token string, timeout time.Duration, offline Offline, proxyURL *url.URL) *Mikan {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultMikanURL
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return &Mikan{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   token,
		offline: offline,
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (m *Mikan) ID() string    { return "mikan" }
func (m *Mikan) Label() string { return "蜜柑" }

func (m *Mikan) Search(ctx context.Context, queryText string) ([]search.Candidate, error) {
	queryText = strings.TrimSpace(queryText)
	if queryText == "" {
		return nil, search.Failure{Code: "invalid_query", Message: "搜索词不能为空", Retryable: false}
	}
	endpoint, err := url.Parse(m.baseURL + "/RSS/Search")
	if err != nil {
		return nil, search.Failure{Code: "source_unconfigured", Message: "蜜柑地址无效", Retryable: false}
	}
	query := endpoint.Query()
	query.Set("searchstr", queryText)
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, search.Failure{Code: "source_unconfigured", Message: "无法创建蜜柑请求", Retryable: false}
	}
	request.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	request.Header.Set("User-Agent", "Media-Hub/mikan")
	if m.token != "" {
		request.Header.Set("Authorization", "Bearer "+m.token)
	}
	response, err := m.client.Do(request)
	if err != nil {
		return nil, search.Failure{Code: "source_unavailable", Message: "蜜柑暂时不可用", Retryable: true}
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, search.Failure{Code: "source_unauthorized", Message: "蜜柑鉴权失败", Retryable: false}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, search.Failure{Code: "source_unavailable", Message: "蜜柑搜索失败", Retryable: true}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20+1))
	if err != nil || len(body) > 4<<20 {
		return nil, search.Failure{Code: "invalid_source_response", Message: "蜜柑响应无效", Retryable: false}
	}
	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, search.Failure{Code: "invalid_source_response", Message: "蜜柑 RSS 无法解析", Retryable: false}
	}
	candidates := make([]search.Candidate, 0, len(feed.Channel.Items))
	seen := map[string]struct{}{}
	for _, item := range feed.Channel.Items {
		candidate, ok := mikanCandidate(item)
		if !ok {
			continue
		}
		if _, exists := seen[candidate.ID]; exists {
			continue
		}
		seen[candidate.ID] = struct{}{}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func (m *Mikan) StartTransfer(ctx context.Context, input search.TransferRequest) (search.TransferResult, error) {
	var reference mikanReference
	if json.Unmarshal([]byte(input.Reference), &reference) != nil || strings.TrimSpace(reference.URL) == "" {
		return search.TransferResult{}, search.Failure{Code: "invalid_source_reference", Message: "蜜柑资源引用无效", Retryable: false}
	}
	if m.offline == nil {
		return search.TransferResult{}, search.Failure{Code: "source_unconfigured", Message: "115 离线转存未配置", Retryable: false}
	}
	if err := m.offline.AddOfflineURLs(ctx, input.DestinationID, []string{reference.URL}); err != nil {
		var uncertain interface{ SubmissionUncertain() bool }
		if errors.As(err, &uncertain) && uncertain.SubmissionUncertain() {
			return search.TransferResult{}, search.Failure{Code: "source_submission_unknown", Message: "115 离线转存结果未知，需要人工确认", Retryable: true}
		}
		return search.TransferResult{}, search.Failure{Code: "source_unavailable", Message: "蜜柑转存到 115 失败", Retryable: automaticWriteRetryAllowed(err)}
	}
	return search.TransferResult{
		OperationID: input.IdempotencyKey,
		Status:      "completed",
		FileID:      input.DestinationID,
		Path:        reference.Title,
		IsFile:      false,
	}, nil
}

func (m *Mikan) TransferStatus(_ context.Context, _ int64, operationID string) (search.TransferResult, error) {
	return search.TransferResult{}, search.Failure{Code: "invalid_source_response", Message: fmt.Sprintf("蜜柑转存没有可查询的异步状态: %s", operationID), Retryable: false}
}

func mikanCandidate(item rssItem) (search.Candidate, bool) {
	title := strings.TrimSpace(item.Title)
	link := strings.TrimSpace(item.Enclosure.URL)
	if link == "" {
		link = strings.TrimSpace(item.Link)
	}
	if title == "" || link == "" {
		return search.Candidate{}, false
	}
	release := search.ReleaseFacts{
		Resolution: mikanNormalizedResolution(title),
		VideoCodec: mikanNormalizedCodec(title),
		SizeBytes:  mikanSize(item.Enclosure.Length),
	}
	if release.Resolution == "" {
		release.Resolution = "1080p"
	}
	if release.VideoCodec == "" {
		release.VideoCodec = "AVC"
	}
	mediaType := "series"
	season, episode := 1, 1
	if mikanMovieMark.MatchString(title) {
		mediaType = "movie"
		season, episode = 0, 0
	} else if captured := mikanEpisode.FindStringSubmatch(title); len(captured) == 4 {
		if captured[1] != "" {
			season = atoiDefault(captured[1], 1)
			episode = atoiDefault(captured[2], 1)
		} else {
			episode = atoiDefault(captured[3], 1)
		}
	}
	reference, err := json.Marshal(mikanReference{Title: title, URL: link})
	if err != nil {
		return search.Candidate{}, false
	}
	digest := sha256.Sum256([]byte(link))
	id := "mikan-" + hex.EncodeToString(digest[:8])
	return search.Candidate{
		ID: titleID(id, title), Title: title, Year: 0,
		Season: season, EpisodeStart: episode, EpisodeEnd: episode, MediaType: mediaType,
		Source: "蜜柑", SourceID: "mikan", SourceRef: string(reference),
		Release: release, TransferState: "available",
	}, true
}

func titleID(id, title string) string {
	if len([]rune(title)) > 300 {
		return id
	}
	return id
}

func mikanNormalizedResolution(title string) string {
	value := strings.ToLower(mikanResolution.FindString(title))
	if value == "4k" {
		return "2160p"
	}
	return value
}

func mikanNormalizedCodec(title string) string {
	value := strings.ToLower(strings.NewReplacer(".", "", " ", "").Replace(mikanCodec.FindString(title)))
	switch value {
	case "hevc", "x265", "h265":
		return "HEVC"
	case "av1":
		return "AV1"
	case "avc", "x264", "h264":
		return "AVC"
	default:
		return ""
	}
}

func mikanSize(raw string) int64 {
	parsed, _ := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if parsed < 0 {
		return 0
	}
	return parsed
}

func atoiDefault(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
