package emby

import (
	"context"
	"errors"
	"strings"

	"media-hub/backend/internal/integration"
)

// PlaybackChecker reports deployment-level QMediaSync emby302 readiness separately from ordinary Emby health.
type PlaybackChecker struct {
	client *Client
}

func NewPlaybackChecker(client *Client) *PlaybackChecker {
	return &PlaybackChecker{client: client}
}

func (checker *PlaybackChecker) Check(ctx context.Context) integration.Health {
	health := integration.Health{ID: "emby-playback", Label: "Media3 播放入口"}
	if checker == nil || checker.client == nil || strings.TrimSpace(checker.client.playbackBaseURL) == "" {
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未配置 QMediaSync 播放入口"
		return health
	}
	configuration := checker.client.configuration()
	if configuration.apiKey == "" {
		health.Status = integration.StatusDegraded
		health.Detail = "播放入口可用性未知，缺少 Emby API Key"
		return health
	}
	playbackConfiguration := configuration
	playbackConfiguration.baseURL = checker.client.playbackBaseURL
	if _, err := checker.client.readServerInfo(ctx, playbackConfiguration, "System/Info", true); err == nil {
		health.Status = integration.StatusHealthy
		health.Detail = "QMediaSync 播放入口在线"
		return health
	} else if errors.Is(err, ErrUnauthorized) {
		health.Status = integration.StatusDegraded
		health.Detail = "播放入口可达，但 Emby 鉴权失败"
		return health
	}
	health.Status = integration.StatusUnavailable
	health.Detail = "无法连接 QMediaSync 播放入口"
	return health
}

var _ integration.Checker = (*PlaybackChecker)(nil)
