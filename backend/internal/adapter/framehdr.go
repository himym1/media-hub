package adapter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/net/html"

	"media-hub/backend/internal/mediaidentity"
	"media-hub/backend/internal/search"
)

const (
	defaultFrameHDRURL     = "https://framehdr.com"
	frameHDRMaxPageBytes   = 4 << 20
	frameHDRMaxSearchCards = 8
	frameHDRMaxCandidates  = 100
)

var (
	frameHDRDetailCall = regexp.MustCompile(`(?i)location\.href\s*=\s*['"]detail\.php\?id=([0-9]{1,12})['"]`)
	frameHDRCopyCall   = regexp.MustCompile(`(?is)^\s*copyToClipboard\(\s*'([^'\r\n]+)'`)
	frameHDRShareCode  = regexp.MustCompile(`^[A-Za-z0-9]{4,64}$`)
	frameHDRAccessCode = regexp.MustCompile(`^[A-Za-z0-9]{0,8}$`)
)

type FrameHDR struct {
	baseURL  string
	account  string
	password string
	client   *http.Client
	receiver ShareReceiver
	loginMu  sync.Mutex
}

type frameHDRCard struct {
	Title      string
	Year       int
	MediaType  string
	DetailPath string
}

type frameHDRReference struct {
	Title       string `json:"title"`
	ShareCode   string `json:"shareCode"`
	ReceiveCode string `json:"receiveCode,omitempty"`
}

func NewFrameHDR(baseURL, account, password string, timeout time.Duration, receiver ShareReceiver, proxyURL *url.URL) *FrameHDR {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultFrameHDRURL
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	jar, _ := cookiejar.New(nil)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return &FrameHDR{
		baseURL: baseURL, account: strings.TrimSpace(account), password: password, receiver: receiver,
		client: &http.Client{
			Timeout: timeout, Transport: transport, Jar: jar,
			CheckRedirect: func(request *http.Request, via []*http.Request) error {
				if len(via) >= 4 || !sameOrigin(baseURL, request.URL) {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
	}
}

func (f *FrameHDR) ID() string    { return "framehdr" }
func (f *FrameHDR) Label() string { return "帧影" }

func (f *FrameHDR) Search(ctx context.Context, queryText string) ([]search.Candidate, error) {
	queryText = strings.TrimSpace(queryText)
	if queryText == "" || len([]rune(queryText)) > 200 {
		return nil, search.Failure{Code: "invalid_query", Message: "搜索词无效", Retryable: false}
	}
	if f.account == "" || f.password == "" {
		return nil, search.Failure{Code: "source_unconfigured", Message: "帧影账号密码未配置", Retryable: false}
	}
	if err := f.ensureLogin(ctx); err != nil {
		return nil, err
	}
	searchPath := "/search.php?q=" + url.QueryEscape(queryText)
	body, err := f.fetchAuthenticated(ctx, searchPath, frameHDRMaxPageBytes)
	if err != nil {
		return nil, err
	}
	cards, err := parseFrameHDRSearch(body, queryText)
	if err != nil {
		return nil, search.Failure{Code: "invalid_source_response", Message: "帧影搜索响应无法解析", Retryable: false}
	}
	if len(cards) > frameHDRMaxSearchCards {
		cards = cards[:frameHDRMaxSearchCards]
	}
	type detailOutcome struct {
		index      int
		candidates []search.Candidate
		err        error
	}
	outcomes := make(chan detailOutcome, len(cards))
	for index, card := range cards {
		go func() {
			detail, fetchErr := f.fetchAuthenticated(ctx, card.DetailPath, frameHDRMaxPageBytes)
			if fetchErr != nil {
				outcomes <- detailOutcome{index: index, err: fetchErr}
				return
			}
			parsed, parseErr := parseFrameHDRDetail(detail, card)
			outcomes <- detailOutcome{index: index, candidates: parsed, err: parseErr}
		}()
	}
	ordered := make([]detailOutcome, len(cards))
	for range cards {
		outcome := <-outcomes
		ordered[outcome.index] = outcome
	}
	results := make([]search.Candidate, 0)
	failedDetails := 0
	for _, outcome := range ordered {
		if outcome.err != nil {
			failedDetails++
			continue
		}
		for _, candidate := range outcome.candidates {
			results = append(results, candidate)
			if len(results) == frameHDRMaxCandidates {
				return results, nil
			}
		}
	}
	if len(results) == 0 && failedDetails > 0 {
		return nil, search.Failure{Code: "source_unavailable", Message: "帧影资源详情暂时不可用", Retryable: true}
	}
	return results, nil
}

func (f *FrameHDR) StartTransfer(ctx context.Context, input search.TransferRequest) (search.TransferResult, error) {
	var reference frameHDRReference
	if json.Unmarshal([]byte(input.Reference), &reference) != nil || strings.TrimSpace(reference.Title) == "" || !frameHDRShareCode.MatchString(reference.ShareCode) || !frameHDRAccessCode.MatchString(reference.ReceiveCode) {
		return search.TransferResult{}, search.Failure{Code: "invalid_source_reference", Message: "帧影资源引用无效", Retryable: false}
	}
	if f.receiver == nil {
		return search.TransferResult{}, search.Failure{Code: "source_unconfigured", Message: "115 分享接收未配置", Retryable: false}
	}
	if inspector, ok := f.receiver.(ShareInspector); ok {
		videoNames, _, err := inspector.InspectShare(ctx, reference.ShareCode, reference.ReceiveCode)
		if err != nil {
			return search.TransferResult{}, search.Failure{Code: "source_unavailable", Message: "无法校验分享内容", Retryable: automaticWriteRetryAllowed(err)}
		}
		if mediaidentity.ShareContentConflictsWithMediaType(input.MediaType, videoNames) {
			return search.TransferResult{}, search.Failure{Code: "source_identity_mismatch", Message: "分享内容像是电视剧分集，请按剧集重新搜索", Retryable: false}
		}
	}
	destinationID, storageTitle, err := ensureTransferDestination(ctx, asFolderEnsurer(f.receiver), input.DestinationID, input.Title, reference.Title)
	if err != nil {
		return search.TransferResult{}, transferFolderFailure(err)
	}
	if err := f.receiver.ReceiveShare(ctx, destinationID, reference.ShareCode, reference.ReceiveCode, nil); err != nil {
		var uncertain interface{ SubmissionUncertain() bool }
		if errors.As(err, &uncertain) && uncertain.SubmissionUncertain() {
			return search.TransferResult{}, search.Failure{Code: "source_submission_unknown", Message: "115 分享接收结果未知，需要人工确认", Retryable: true}
		}
		return search.TransferResult{}, search.Failure{Code: "source_unavailable", Message: "帧影资源接收到 115 失败", Retryable: automaticWriteRetryAllowed(err)}
	}
	return search.TransferResult{
		OperationID: input.IdempotencyKey, Status: "completed", FileID: destinationID, Path: storageTitle, IsFile: false,
	}, nil
}

func (f *FrameHDR) TransferStatus(_ context.Context, _ int64, operationID string) (search.TransferResult, error) {
	return search.TransferResult{}, search.Failure{Code: "invalid_source_response", Message: fmt.Sprintf("帧影转存没有可查询的异步状态: %s", operationID), Retryable: false}
}

func (f *FrameHDR) ensureLogin(ctx context.Context) error {
	f.loginMu.Lock()
	defer f.loginMu.Unlock()
	body, err := f.fetch(ctx, "/", frameHDRMaxPageBytes, nil)
	if err == nil && frameHDRLoggedIn(body) {
		return nil
	}
	return f.login(ctx)
}

func (f *FrameHDR) login(ctx context.Context) error {
	page, err := f.fetch(ctx, "/login.php", frameHDRMaxPageBytes, nil)
	if err != nil {
		return err
	}
	values, err := frameHDRLoginFields(page)
	if err != nil {
		return search.Failure{Code: "invalid_source_response", Message: "帧影登录页面无法解析", Retryable: false}
	}
	values.Set("login_mode", "password")
	values.Set("username", f.account)
	values.Set("password", f.password)
	values.Set("remember_me", "on")
	headers := http.Header{"Content-Type": {"application/x-www-form-urlencoded"}, "Referer": {f.baseURL + "/login.php"}, "Origin": {f.baseURL}}
	body, err := f.fetch(ctx, "/login.php", frameHDRMaxPageBytes, &frameHDRPost{Values: values, Headers: headers})
	if err != nil {
		return err
	}
	if !frameHDRLoggedIn(body) {
		body, err = f.fetch(ctx, "/", frameHDRMaxPageBytes, nil)
		if err != nil {
			return err
		}
	}
	if !frameHDRLoggedIn(body) {
		return search.Failure{Code: "source_unauthorized", Message: "帧影登录失败", Retryable: false}
	}
	return nil
}

type frameHDRPost struct {
	Values  url.Values
	Headers http.Header
}

func (f *FrameHDR) fetchAuthenticated(ctx context.Context, path string, limit int64) ([]byte, error) {
	body, err := f.fetch(ctx, path, limit, nil)
	if err != nil || !frameHDRLoginRequired(body) {
		return body, err
	}
	f.loginMu.Lock()
	loginErr := f.login(ctx)
	f.loginMu.Unlock()
	if loginErr != nil {
		return nil, loginErr
	}
	return f.fetch(ctx, path, limit, nil)
}

func (f *FrameHDR) fetch(ctx context.Context, rawPath string, limit int64, post *frameHDRPost) ([]byte, error) {
	endpoint, err := url.Parse(f.baseURL + rawPath)
	if err != nil || endpoint.Scheme != "https" || !sameOrigin(f.baseURL, endpoint) {
		return nil, search.Failure{Code: "source_unconfigured", Message: "帧影地址无效", Retryable: false}
	}
	method := http.MethodGet
	var body io.Reader
	if post != nil {
		method = http.MethodPost
		body = strings.NewReader(post.Values.Encode())
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, search.Failure{Code: "source_unconfigured", Message: "无法创建帧影请求", Retryable: false}
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/124.0.0.0 Safari/537.36")
	if post != nil {
		for key, values := range post.Headers {
			for _, value := range values {
				request.Header.Add(key, value)
			}
		}
	} else {
		request.Header.Set("Referer", f.baseURL+"/")
	}
	response, err := f.client.Do(request)
	if err != nil {
		return nil, search.Failure{Code: "source_unavailable", Message: "帧影暂时不可用", Retryable: true}
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		location, parseErr := response.Location()
		if parseErr != nil || !sameOrigin(f.baseURL, location) {
			return nil, search.Failure{Code: "invalid_source_response", Message: "帧影登录跳转无效", Retryable: false}
		}
		if post != nil {
			return nil, nil
		}
		next := location.EscapedPath()
		if location.RawQuery != "" {
			next += "?" + location.RawQuery
		}
		if next == rawPath {
			return nil, search.Failure{Code: "invalid_source_response", Message: "帧影跳转无效", Retryable: false}
		}
		return f.fetch(ctx, next, limit, nil)
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, search.Failure{Code: "source_unauthorized", Message: "帧影鉴权失败", Retryable: false}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, search.Failure{Code: "source_unavailable", Message: "帧影请求失败", Retryable: true}
	}
	result, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || int64(len(result)) > limit {
		return nil, search.Failure{Code: "invalid_source_response", Message: "帧影响应无效", Retryable: false}
	}
	return result, nil
}

func parseFrameHDRSearch(body []byte, queryText string) ([]frameHDRCard, error) {
	root, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	cards := make([]frameHDRCard, 0)
	walkHTML(root, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "div" || !hasHTMLClass(node, "resource-card") {
			return
		}
		matched := frameHDRDetailCall.FindStringSubmatch(htmlAttr(node, "onclick"))
		if len(matched) != 2 {
			return
		}
		titleNode := findHTMLNode(node, func(child *html.Node) bool { return hasHTMLClass(child, "card-title") })
		categoryNode := findHTMLNode(node, func(child *html.Node) bool { return hasHTMLClass(child, "category-badge-overlay") })
		metaNode := findHTMLNode(node, func(child *html.Node) bool {
			return hasHTMLClass(child, "meta-info") || hasHTMLClass(child, "card-meta")
		})
		title := strings.TrimSpace(htmlText(titleNode))
		if title == "" || len([]rune(title)) > 300 {
			return
		}
		if frameHDRTitleContains(title, queryText) {
			title = strings.TrimSpace(queryText)
		}
		cards = append(cards, frameHDRCard{
			Title: title, Year: sidhubYearValue(htmlText(metaNode)), MediaType: frameHDRMediaType(htmlText(categoryNode)),
			DetailPath: "/detail.php?id=" + matched[1],
		})
	})
	return cards, nil
}

func parseFrameHDRDetail(body []byte, card frameHDRCard) ([]search.Candidate, error) {
	root, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	results := make([]search.Candidate, 0)
	seen := make(map[string]struct{})
	walkHTML(root, func(node *html.Node) {
		if len(results) >= frameHDRMaxCandidates || node.Type != html.ElementNode || node.Data != "div" || !hasHTMLClass(node, "download-link") {
			return
		}
		disk := parentHTMLClass(node, "disk-content")
		if disk == nil || !strings.Contains(strings.ToLower(htmlAttr(disk, "data-disk")+" "+htmlText(findHTMLNode(disk, func(child *html.Node) bool {
			return hasHTMLClass(child, "disk-title") || hasHTMLClass(child, "tab-title")
		}))), "115") {
			return
		}
		button := findHTMLNode(node, func(child *html.Node) bool {
			return child.Type == html.ElementNode && htmlAttr(child, "onclick") != "" && strings.Contains(htmlAttr(child, "onclick"), "copyToClipboard")
		})
		if button == nil {
			return
		}
		linkID := strings.TrimSpace(htmlAttr(node, "data-link-id"))
		if _, err := strconv.ParseUint(linkID, 10, 64); err != nil {
			return
		}
		matched := frameHDRCopyCall.FindStringSubmatch(htmlAttr(button, "onclick"))
		if len(matched) != 2 || len(matched[1]) > 4096 || strings.Contains(matched[1], `\`) {
			return
		}
		shareCode, receiveCode, parseErr := parseFrameHDRShareURL(matched[1])
		if parseErr != nil {
			return
		}
		nameNode := findHTMLNode(node, func(child *html.Node) bool {
			return hasHTMLClass(child, "link-name") || hasHTMLClass(child, "link-description") || hasHTMLClass(child, "description")
		})
		releaseTitle := strings.TrimSpace(htmlText(nameNode))
		if releaseTitle == "" {
			releaseTitle = card.Title
		}
		if len([]rune(releaseTitle)) > 300 {
			return
		}
		key := linkID + ":" + shareCode
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		reference, marshalErr := json.Marshal(frameHDRReference{Title: releaseTitle, ShareCode: shareCode, ReceiveCode: receiveCode})
		if marshalErr != nil {
			return
		}
		season, episodeStart, episodeEnd := 0, 0, 0
		if card.MediaType == "series" {
			season, episodeStart, episodeEnd = sidhubEpisodeRange(releaseTitle)
		}
		results = append(results, search.Candidate{
			ID: "framehdr-" + frameHDRReferenceID(key), Title: card.Title, Year: card.Year,
			Season: season, EpisodeStart: episodeStart, EpisodeEnd: episodeEnd, MediaType: card.MediaType,
			SourceID: "framehdr", SourceRef: string(reference), TransferState: "available",
			Release: search.ReleaseFacts{
				Resolution: mikanNormalizedResolution(releaseTitle), VideoCodec: mikanNormalizedCodec(releaseTitle),
				DynamicRange: normalizedSidhubHDR(releaseTitle), SizeBytes: firstReleaseSizeBytes(htmlText(node), releaseTitle),
			},
		})
	})
	return results, nil
}

func frameHDRLoginFields(body []byte) (url.Values, error) {
	root, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	form := findHTMLNode(root, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "form" && (htmlAttr(node, "id") == "loginForm" || findHTMLNode(node, func(child *html.Node) bool {
			return child.Type == html.ElementNode && child.Data == "input" && htmlAttr(child, "name") == "password"
		}) != nil)
	})
	if form == nil {
		return nil, errors.New("login form missing")
	}
	values := url.Values{}
	walkHTML(form, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "input" {
			return
		}
		name := strings.TrimSpace(htmlAttr(node, "name"))
		typeName := strings.ToLower(strings.TrimSpace(htmlAttr(node, "type")))
		if name != "" && (typeName == "hidden" || typeName == "") {
			values.Set(name, htmlAttr(node, "value"))
		}
	})
	return values, nil
}

func frameHDRLoginRequired(body []byte) bool {
	text := strings.ToLower(string(body))
	return strings.Contains(text, "login.php") && strings.Contains(text, `name="password"`) || strings.Contains(text, "登录后可查看下载链接")
}

func frameHDRLoggedIn(body []byte) bool {
	if frameHDRLoginRequired(body) {
		return false
	}
	text := strings.ToLower(string(body))
	return strings.Contains(text, "logout") || strings.Contains(text, "user/index.php") || strings.Contains(text, "checkinbtn")
}

func frameHDRMediaType(category string) string {
	category = strings.ToLower(strings.TrimSpace(category))
	switch {
	case strings.Contains(category, "电影"), strings.Contains(category, "movie"):
		return "movie"
	case strings.Contains(category, "剧"), strings.Contains(category, "动漫"), strings.Contains(category, "番"):
		return "series"
	default:
		return ""
	}
}

func frameHDRTitleContains(candidateTitle, queryText string) bool {
	candidateRunes := []rune(strings.ToLower(strings.TrimSpace(candidateTitle)))
	queryRunes := []rune(strings.ToLower(strings.TrimSpace(queryText)))
	if len(queryRunes) < 2 || (len(queryRunes) < 3 && allASCIIAdapter(queryRunes)) {
		return false
	}
	for start := 0; start+len(queryRunes) <= len(candidateRunes); start++ {
		if string(candidateRunes[start:start+len(queryRunes)]) != string(queryRunes) {
			continue
		}
		if start > 0 && (unicode.IsLetter(candidateRunes[start-1]) || unicode.IsDigit(candidateRunes[start-1])) {
			continue
		}
		end := start + len(queryRunes)
		if end < len(candidateRunes) && (unicode.IsLetter(candidateRunes[end]) || unicode.IsDigit(candidateRunes[end])) {
			continue
		}
		return true
	}
	return false
}

func allASCIIAdapter(values []rune) bool {
	for _, value := range values {
		if value > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func parseFrameHDRShareURL(raw string) (string, string, error) {
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
	if !frameHDRAccessCode.MatchString(receiveCode) {
		return "", "", errors.New("invalid receive code")
	}
	return shareCode, receiveCode, nil
}

func frameHDRReferenceID(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:8])
}

func parentHTMLClass(node *html.Node, className string) *html.Node {
	for current := node.Parent; current != nil; current = current.Parent {
		if current.Type == html.ElementNode && hasHTMLClass(current, className) {
			return current
		}
	}
	return nil
}
