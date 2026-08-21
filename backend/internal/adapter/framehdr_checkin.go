package adapter

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"

	"media-hub/backend/internal/search"
)

var (
	frameHDRCheckInCall = regexp.MustCompile(`(?i)(?:location\.href|fetch|ajax\.(?:get|post)|\$\.(?:get|post)|axios\.(?:get|post))\s*[=(]\s*['"]([^'"]+)['"]`)
	frameHDRCheckInHint = regexp.MustCompile(`(?i)(?:attendance|checkin|check_in|signin|sign_in)`)
)

type frameHDRCheckInAction struct {
	Path   string
	Method string
}

func (f *FrameHDR) CheckIn(ctx context.Context) (search.CheckInResult, error) {
	if f.account == "" || f.password == "" {
		return search.CheckInResult{State: "skipped", Message: "帧影账号未配置"}, nil
	}
	if err := f.ensureLogin(ctx); err != nil {
		return search.CheckInResult{}, err
	}
	var lastErr error
	sawButton := false
	for _, path := range []string{"/", "/user/index.php"} {
		body, err := f.fetchAuthenticated(ctx, path, frameHDRMaxPageBytes)
		if err != nil {
			lastErr = err
			continue
		}
		if state, message, ok := checkInOutcome(string(body)); ok {
			return search.CheckInResult{State: state, Message: message}, nil
		}
		if frameHDRHasCheckInControl(body) {
			sawButton = true
		}
		if action, ok := parseFrameHDRCheckInAction(body); ok {
			return f.submitCheckIn(ctx, action)
		}
	}
	if lastErr != nil && !sawButton {
		return search.CheckInResult{}, lastErr
	}
	if sawButton {
		for _, path := range []string{"/attendance.php", "/checkin.php", "/user/checkin.php"} {
			result, err := f.submitCheckIn(ctx, frameHDRCheckInAction{Path: path, Method: http.MethodGet})
			if err == nil {
				return result, nil
			}
			result, err = f.submitCheckIn(ctx, frameHDRCheckInAction{Path: path, Method: http.MethodPost})
			if err == nil {
				return result, nil
			}
		}
	}
	return search.CheckInResult{}, search.Failure{Code: "checkin_unavailable", Message: "未找到帧影签到入口", Retryable: true}
}

func (f *FrameHDR) submitCheckIn(ctx context.Context, action frameHDRCheckInAction) (search.CheckInResult, error) {
	var post *frameHDRPost
	if action.Method == http.MethodPost {
		post = &frameHDRPost{
			Values:  url.Values{},
			Headers: http.Header{"Content-Type": {"application/x-www-form-urlencoded"}, "Referer": {f.baseURL + "/"}, "Origin": {f.baseURL}},
		}
	}
	body, err := f.fetch(ctx, action.Path, frameHDRMaxPageBytes, post)
	if err != nil {
		return search.CheckInResult{}, err
	}
	if len(body) == 0 {
		body, err = f.fetchAuthenticated(ctx, "/user/index.php", frameHDRMaxPageBytes)
		if err != nil {
			return search.CheckInResult{}, search.Failure{Code: "checkin_unknown", Message: "帧影签到结果未知，需要确认后重试", Retryable: true}
		}
	}
	if state, message, ok := checkInOutcome(string(body)); ok {
		return search.CheckInResult{State: state, Message: message}, nil
	}
	if frameHDRLoggedIn(body) && !frameHDRHasCheckInControl(body) {
		return search.CheckInResult{State: "completed", Message: "签到成功"}, nil
	}
	return search.CheckInResult{}, search.Failure{Code: "checkin_failed", Message: "帧影签到失败", Retryable: true}
}

func parseFrameHDRCheckInAction(body []byte) (frameHDRCheckInAction, bool) {
	root, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return frameHDRCheckInAction{}, false
	}
	var action frameHDRCheckInAction
	found := false
	walkHTML(root, func(node *html.Node) {
		if found || node.Type != html.ElementNode {
			return
		}
		if node.Data == "form" && frameHDRCheckInHint.MatchString(htmlAttr(node, "action")) {
			if path, ok := frameHDRCheckInPath(htmlAttr(node, "action")); ok {
				method := strings.ToUpper(strings.TrimSpace(htmlAttr(node, "method")))
				if method == "" {
					method = http.MethodPost
				}
				action = frameHDRCheckInAction{Path: path, Method: method}
				found = true
			}
			return
		}
		if !frameHDRIsCheckInControl(node) {
			return
		}
		for _, key := range []string{"href", "data-url", "data-href", "formaction", "action"} {
			if path, ok := frameHDRCheckInPath(htmlAttr(node, key)); ok {
				method := http.MethodGet
				if node.Data == "form" || strings.EqualFold(htmlAttr(node, "method"), http.MethodPost) {
					method = http.MethodPost
				}
				action = frameHDRCheckInAction{Path: path, Method: method}
				found = true
				return
			}
		}
		if matched := frameHDRCheckInCall.FindStringSubmatch(htmlAttr(node, "onclick")); len(matched) == 2 {
			if path, ok := frameHDRCheckInPath(matched[1]); ok {
				method := http.MethodGet
				if strings.Contains(strings.ToLower(htmlAttr(node, "onclick")), "post") {
					method = http.MethodPost
				}
				action = frameHDRCheckInAction{Path: path, Method: method}
				found = true
			}
		}
	})
	return action, found
}

func frameHDRHasCheckInControl(body []byte) bool {
	root, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return strings.Contains(strings.ToLower(string(body)), "checkinbtn")
	}
	return findHTMLNode(root, frameHDRIsCheckInControl) != nil
}

func frameHDRIsCheckInControl(node *html.Node) bool {
	if node == nil || node.Type != html.ElementNode {
		return false
	}
	identity := strings.ToLower(strings.Join([]string{
		htmlAttr(node, "id"), htmlAttr(node, "class"), htmlAttr(node, "name"), htmlText(node),
	}, " "))
	return strings.Contains(identity, "checkin") || strings.Contains(identity, "签到") || strings.Contains(identity, "attendance")
}

func frameHDRCheckInPath(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "javascript:") || strings.HasPrefix(raw, "#") {
		return "", false
	}
	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Path == "" {
			return "", false
		}
		raw = parsed.Path
		if parsed.RawQuery != "" {
			raw += "?" + parsed.RawQuery
		}
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	if strings.Contains(raw, "..") || !frameHDRCheckInHint.MatchString(raw) {
		return "", false
	}
	return raw, true
}

func checkInOutcome(text string) (string, string, bool) {
	compact := strings.ToLower(strings.Join(strings.Fields(text), ""))
	switch {
	case strings.Contains(compact, "今日已签"), strings.Contains(compact, "已经签到"), strings.Contains(compact, "已签到"), strings.Contains(compact, "alreadychecked"):
		return "completed", "今日已签到", true
	case strings.Contains(compact, "签到成功"), strings.Contains(compact, "签到完成"), strings.Contains(compact, "checkinsuccess"):
		return "completed", "签到成功", true
	default:
		return "", "", false
	}
}
