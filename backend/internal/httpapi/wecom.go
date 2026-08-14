package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"media-hub/backend/internal/wecom"
)

type notificationTestResponse struct {
	Status string `json:"status"`
}

func (h *handler) testWeComNotification(w http.ResponseWriter, r *http.Request) {
	if h.dependencies.WeComTester == nil || !h.dependencies.WeComTester.Configured() {
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/notification-unavailable",
			Title: "企业微信通知尚未配置", Status: http.StatusServiceUnavailable,
			Code: "notification_unavailable",
		})
		return
	}
	content := fmt.Sprintf("Media Hub 通知测试\n版本：%s\n时间：%s", h.version, h.now().UTC().Format("2006-01-02 15:04:05 UTC"))
	unknown, err := h.dependencies.WeComTester.Send(r.Context(), content)
	if err != nil {
		if unknown || errors.Is(err, wecom.ErrSubmissionUnknown) {
			writeProblem(w, problem{
				Type:  "https://media-hub.local/problems/notification-result-unknown",
				Title: "企业微信通知结果未知，请先检查客户端再决定是否重试", Status: http.StatusBadGateway,
				Code: "notification_result_unknown",
			})
			return
		}
		writeProblem(w, problem{
			Type:  "https://media-hub.local/problems/notification-rejected",
			Title: "企业微信拒绝了测试通知", Status: http.StatusBadGateway,
			Code: "notification_rejected",
		})
		return
	}
	writeJSON(w, http.StatusOK, notificationTestResponse{Status: "sent"})
}
