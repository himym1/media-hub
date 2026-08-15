package adapter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"media-hub/backend/internal/search"
)

const (
	defaultSidhubURL      = "https://sidhub.cc"
	sidhubMaxSearchCards  = 6
	sidhubMaxCandidates   = 50
	sidhubMaxPageBytes    = 4 << 20
	sidhubMaxLinkPageSize = 1 << 20
)

var (
	sidhubMoviePath = regexp.MustCompile(`^/movies/[0-9]+/$`)
	sidhubSeedID    = regexp.MustCompile(`^[0-9]+$`)
	sidhubYear      = regexp.MustCompile(`(?:^|[^0-9])((?:19|20)[0-9]{2})(?:[^0-9]|$)`)
	sidhubEpisode   = regexp.MustCompile(`(?i)\bS([0-9]{1,2})E([0-9]{1,4})(?:\s*[-~]\s*E?([0-9]{1,4}))?\b`)
	sidhubData      = regexp.MustCompile(`(?i)(?:const|let|var)\s+data\s*=\s*["']([A-Za-z0-9+/]+={0,2})["']`)
	sidhubBTIH      = regexp.MustCompile(`(?i)^(?:[0-9a-f]{40}|[a-z2-7]{32})$`)
	sidhubSize      = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*([KMGT])(?:i?B)?`)
	sidhubHDR       = regexp.MustCompile(`(?i)\b(Dolby[ ._-]?Vision|DV|HDR10\+?|HDR|HLG)\b`)
)

type Sidhub struct {
	baseURL string
	client  *http.Client
	offline Offline
}

type sidhubCard struct {
	Title      string
	FullTitle  string
	Year       int
	DetailPath string
}

type sidhubReference struct {
	Title    string `json:"title"`
	LinkPath string `json:"linkPath"`
}

func NewSidhub(baseURL string, timeout time.Duration, offline Offline, proxyURL *url.URL) *Sidhub {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultSidhubURL
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return &Sidhub{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
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

func (s *Sidhub) ID() string    { return "sidhub" }
func (s *Sidhub) Label() string { return "Sidhub" }

func (s *Sidhub) Search(ctx context.Context, queryText string) ([]search.Candidate, error) {
	queryText = strings.TrimSpace(queryText)
	if queryText == "" {
		return nil, search.Failure{Code: "invalid_query", Message: "搜索词不能为空", Retryable: false}
	}
	body, err := s.fetch(ctx, s.baseURL+"/s/"+url.PathEscape(queryText)+"/", sidhubMaxPageBytes)
	if err != nil {
		return nil, err
	}
	cards, err := parseSidhubSearch(body)
	if err != nil {
		return nil, search.Failure{Code: "invalid_source_response", Message: "Sidhub 搜索响应无法解析", Retryable: false}
	}
	if len(cards) > sidhubMaxSearchCards {
		cards = cards[:sidhubMaxSearchCards]
	}
	type detailOutcome struct {
		index      int
		candidates []search.Candidate
		err        error
	}
	outcomes := make(chan detailOutcome, len(cards))
	for index, card := range cards {
		go func() {
			detail, fetchErr := s.fetch(ctx, s.baseURL+card.DetailPath, sidhubMaxPageBytes)
			if fetchErr != nil {
				outcomes <- detailOutcome{index: index, err: fetchErr}
				return
			}
			parsed, parseErr := parseSidhubDetail(detail, card)
			outcomes <- detailOutcome{index: index, candidates: parsed, err: parseErr}
		}()
	}
	ordered := make([]detailOutcome, len(cards))
	for range cards {
		outcome := <-outcomes
		ordered[outcome.index] = outcome
	}
	candidates := make([]search.Candidate, 0)
	failedDetails := 0
	for _, outcome := range ordered {
		if outcome.err != nil {
			failedDetails++
			continue
		}
		for _, candidate := range outcome.candidates {
			candidates = append(candidates, candidate)
			if len(candidates) == sidhubMaxCandidates {
				return candidates, nil
			}
		}
	}
	if len(candidates) == 0 && failedDetails > 0 {
		return nil, search.Failure{Code: "source_unavailable", Message: "Sidhub 资源详情暂时不可用", Retryable: true}
	}
	return candidates, nil
}

func (s *Sidhub) StartTransfer(ctx context.Context, input search.TransferRequest) (search.TransferResult, error) {
	var reference sidhubReference
	if json.Unmarshal([]byte(input.Reference), &reference) != nil || strings.TrimSpace(reference.Title) == "" {
		return search.TransferResult{}, search.Failure{Code: "invalid_source_reference", Message: "Sidhub 资源引用无效", Retryable: false}
	}
	endpoint, err := s.linkEndpoint(reference.LinkPath)
	if err != nil {
		return search.TransferResult{}, search.Failure{Code: "invalid_source_reference", Message: "Sidhub 资源引用无效", Retryable: false}
	}
	body, err := s.fetch(ctx, endpoint, sidhubMaxLinkPageSize)
	if err != nil {
		return search.TransferResult{}, err
	}
	magnet, err := decodeSidhubMagnet(body)
	if err != nil {
		return search.TransferResult{}, search.Failure{Code: "invalid_source_response", Message: "Sidhub 磁力链接无效", Retryable: false}
	}
	if s.offline == nil {
		return search.TransferResult{}, search.Failure{Code: "source_unconfigured", Message: "115 离线转存未配置", Retryable: false}
	}
	if err := s.offline.AddOfflineURLs(ctx, input.DestinationID, []string{magnet}); err != nil {
		var uncertain interface{ SubmissionUncertain() bool }
		if errors.As(err, &uncertain) && uncertain.SubmissionUncertain() {
			return search.TransferResult{}, search.Failure{Code: "source_submission_unknown", Message: "115 离线转存结果未知，需要人工确认", Retryable: true}
		}
		return search.TransferResult{}, search.Failure{Code: "source_unavailable", Message: "Sidhub 转存到 115 失败", Retryable: automaticWriteRetryAllowed(err)}
	}
	return search.TransferResult{
		OperationID: input.IdempotencyKey,
		Status:      "completed",
		FileID:      input.DestinationID,
		Path:        reference.Title,
		IsFile:      false,
	}, nil
}

func (s *Sidhub) TransferStatus(_ context.Context, _ int64, operationID string) (search.TransferResult, error) {
	return search.TransferResult{}, search.Failure{Code: "invalid_source_response", Message: fmt.Sprintf("Sidhub 转存没有可查询的异步状态: %s", operationID), Retryable: false}
}

func (s *Sidhub) fetch(ctx context.Context, endpoint string, limit int64) ([]byte, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || !sameOrigin(s.baseURL, parsed) {
		return nil, search.Failure{Code: "source_unconfigured", Message: "Sidhub 地址无效", Retryable: false}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, search.Failure{Code: "source_unconfigured", Message: "无法创建 Sidhub 请求", Retryable: false}
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/124.0.0.0 Safari/537.36")
	response, err := s.client.Do(request)
	if err != nil {
		return nil, search.Failure{Code: "source_unavailable", Message: "Sidhub 暂时不可用", Retryable: true}
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, search.Failure{Code: "source_unauthorized", Message: "Sidhub 拒绝了匿名访问", Retryable: false}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, search.Failure{Code: "source_unavailable", Message: "Sidhub 请求失败", Retryable: true}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || int64(len(body)) > limit {
		return nil, search.Failure{Code: "invalid_source_response", Message: "Sidhub 响应无效", Retryable: false}
	}
	return body, nil
}

func (s *Sidhub) linkEndpoint(rawPath string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawPath))
	if err != nil || parsed.IsAbs() || parsed.Path != "/link_start/" || parsed.Fragment != "" {
		return "", errors.New("invalid link path")
	}
	seedID := parsed.Query().Get("seed_id")
	if !sidhubSeedID.MatchString(seedID) {
		return "", errors.New("invalid seed id")
	}
	endpoint, _ := url.Parse(s.baseURL + "/link_start/")
	query := endpoint.Query()
	query.Set("seed_id", seedID)
	endpoint.RawQuery = query.Encode()
	return endpoint.String(), nil
}

func parseSidhubSearch(body []byte) ([]sidhubCard, error) {
	root, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	cards := make([]sidhubCard, 0)
	walkHTML(root, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "div" || !hasHTMLClass(node, "cover") {
			return
		}
		image := findHTMLNode(node, func(child *html.Node) bool {
			return child.Type == html.ElementNode && child.Data == "a" && hasHTMLClass(child, "image") && sidhubMoviePath.MatchString(htmlAttr(child, "href"))
		})
		heading := findHTMLNode(node, func(child *html.Node) bool {
			return child.Type == html.ElementNode && child.Data == "h2"
		})
		if image == nil || heading == nil {
			return
		}
		title := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(htmlText(heading)), "#"))
		if title == "" || len([]rune(title)) > 300 {
			return
		}
		cards = append(cards, sidhubCard{
			Title:      title,
			FullTitle:  strings.TrimSpace(htmlAttr(image, "title")),
			Year:       sidhubYearValue(htmlText(node)),
			DetailPath: htmlAttr(image, "href"),
		})
	})
	return cards, nil
}

func parseSidhubDetail(body []byte, card sidhubCard) ([]search.Candidate, error) {
	root, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	canonicalTitle := card.Title
	if heading := findHTMLNode(root, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "h1" && htmlAttr(node, "id") == "cover"
	}); heading != nil {
		value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(htmlText(heading)), "#"))
		if card.FullTitle != "" && strings.HasPrefix(value, card.FullTitle) {
			value = card.Title
		}
		if value != "" && len([]rune(value)) <= 300 {
			canonicalTitle = value
		}
	}
	if card.Year == 0 {
		card.Year = sidhubYearValue(htmlText(root))
	}
	candidates := make([]search.Candidate, 0)
	seen := map[string]struct{}{}
	walkHTML(root, func(node *html.Node) {
		if len(candidates) >= sidhubMaxCandidates || node.Type != html.ElementNode || node.Data != "ul" || !hasHTMLClass(node, "seeds") {
			return
		}
		walkHTML(node, func(link *html.Node) {
			if len(candidates) >= sidhubMaxCandidates || link.Type != html.ElementNode || link.Data != "a" {
				return
			}
			rawPath := htmlAttr(link, "href")
			parsed, parseErr := url.Parse(rawPath)
			if parseErr != nil || parsed.IsAbs() || parsed.Path != "/link_start/" {
				return
			}
			seedID := parsed.Query().Get("seed_id")
			if !sidhubSeedID.MatchString(seedID) {
				return
			}
			if _, exists := seen[seedID]; exists {
				return
			}
			releaseTitle := strings.TrimSpace(htmlAttr(link, "title"))
			if releaseTitle == "" {
				releaseTitle = strings.TrimSpace(htmlText(link))
			}
			if releaseTitle == "" || len([]rune(releaseTitle)) > 300 {
				return
			}
			seen[seedID] = struct{}{}
			season, episodeStart, episodeEnd := sidhubEpisodeRange(releaseTitle)
			linkPath := "/link_start/?seed_id=" + url.QueryEscape(seedID)
			reference, marshalErr := json.Marshal(sidhubReference{Title: releaseTitle, LinkPath: linkPath})
			if marshalErr != nil {
				return
			}
			mediaType := ""
			if episodeStart > 0 {
				mediaType = "series"
			}
			candidates = append(candidates, search.Candidate{
				ID:           "seed-" + seedID,
				Title:        canonicalTitle,
				Year:         card.Year,
				Season:       season,
				EpisodeStart: episodeStart,
				EpisodeEnd:   episodeEnd,
				MediaType:    mediaType,
				SourceID:     "sidhub",
				SourceRef:    string(reference),
				Release: search.ReleaseFacts{
					Resolution:   mikanNormalizedResolution(releaseTitle),
					VideoCodec:   mikanNormalizedCodec(releaseTitle),
					DynamicRange: normalizedSidhubHDR(releaseTitle),
					SizeBytes:    sidhubSizeBytes(htmlText(parentHTMLElement(link, "li"))),
				},
				TransferState: "available",
			})
		})
	})
	return candidates, nil
}

func decodeSidhubMagnet(body []byte) (string, error) {
	matched := sidhubData.FindSubmatch(body)
	if len(matched) != 2 {
		return "", errors.New("missing encoded resource")
	}
	decoded, err := base64.StdEncoding.DecodeString(string(matched[1]))
	if err != nil || len(decoded) == 0 || len(decoded) > 8192 || strings.ContainsAny(string(decoded), "\r\n") {
		return "", errors.New("invalid encoded resource")
	}
	magnet := string(decoded)
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

func sidhubEpisodeRange(title string) (int, int, int) {
	matched := sidhubEpisode.FindStringSubmatch(title)
	if len(matched) != 4 {
		return 0, 0, 0
	}
	season, _ := strconv.Atoi(matched[1])
	start, _ := strconv.Atoi(matched[2])
	end := start
	if matched[3] != "" {
		end, _ = strconv.Atoi(matched[3])
	}
	if season <= 0 || season > 100 || start <= 0 || end < start || end > 10000 {
		return 0, 0, 0
	}
	return season, start, end
}

func sidhubYearValue(value string) int {
	matched := sidhubYear.FindStringSubmatch(value)
	if len(matched) != 2 {
		return 0
	}
	year, _ := strconv.Atoi(matched[1])
	return year
}

func sidhubSizeBytes(value string) int64 {
	matched := sidhubSize.FindStringSubmatch(value)
	if len(matched) != 3 {
		return 0
	}
	number, err := strconv.ParseFloat(matched[1], 64)
	if err != nil || number <= 0 {
		return 0
	}
	power := map[string]int{"K": 1, "M": 2, "G": 3, "T": 4}[strings.ToUpper(matched[2])]
	bytes := number
	for range power {
		bytes *= 1024
	}
	if bytes > float64(^uint64(0)>>1) {
		return 0
	}
	return int64(bytes)
}

func normalizedSidhubHDR(title string) string {
	value := strings.ToUpper(strings.NewReplacer(".", "", "_", "", "-", "", " ", "").Replace(sidhubHDR.FindString(title)))
	switch value {
	case "DOLBYVISION", "DV":
		return "Dolby Vision"
	case "HDR10+":
		return "HDR10+"
	case "HDR10", "HDR":
		return "HDR10"
	case "HLG":
		return "HLG"
	default:
		return ""
	}
}

func walkHTML(node *html.Node, visit func(*html.Node)) {
	visit(node)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walkHTML(child, visit)
	}
}

func findHTMLNode(node *html.Node, predicate func(*html.Node) bool) *html.Node {
	if predicate(node) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if matched := findHTMLNode(child, predicate); matched != nil {
			return matched
		}
	}
	return nil
}

func parentHTMLElement(node *html.Node, tag string) *html.Node {
	for current := node.Parent; current != nil; current = current.Parent {
		if current.Type == html.ElementNode && current.Data == tag {
			return current
		}
	}
	return nil
}

func hasHTMLClass(node *html.Node, className string) bool {
	for _, value := range strings.Fields(htmlAttr(node, "class")) {
		if value == className {
			return true
		}
	}
	return false
}

func htmlAttr(node *html.Node, name string) string {
	if node == nil {
		return ""
	}
	for _, attribute := range node.Attr {
		if attribute.Key == name {
			return strings.TrimSpace(attribute.Val)
		}
	}
	return ""
}

func htmlText(node *html.Node) string {
	if node == nil {
		return ""
	}
	var builder strings.Builder
	walkHTML(node, func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
			builder.WriteByte(' ')
		}
	})
	return strings.Join(strings.Fields(builder.String()), " ")
}

func sameOrigin(baseURL string, endpoint *url.URL) bool {
	base, err := url.Parse(baseURL)
	if err != nil || base.Scheme != "https" || endpoint == nil {
		return false
	}
	return strings.EqualFold(base.Scheme, endpoint.Scheme) && strings.EqualFold(base.Host, endpoint.Host)
}
