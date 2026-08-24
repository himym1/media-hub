package emby

import (
	"context"
	"errors"

	"media-hub/backend/internal/integration"
)

// PlaybackChecker reports the Emby playback probe URL separately from ordinary Emby library health.
type PlaybackChecker struct {
	client *Client
}

func NewPlaybackChecker(client *Client) *PlaybackChecker {
	return &PlaybackChecker{client: client}
}

func (checker *PlaybackChecker) Check(ctx context.Context) integration.Health {
	health := integration.Health{ID: "emby-playback", Label: "Media3 播放入口"}
	if checker == nil || checker.client == nil {
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未配置 Emby 播放入口"
		return health
	}
	configuration := checker.client.configuration()
	if configuration.playbackBaseURL == "" {
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未配置 Emby 播放入口"
		return health
	}
	if configuration.apiKey == "" {
		health.Status = integration.StatusDegraded
		health.Detail = "播放入口可用性未知，缺少 Emby API Key"
		return health
	}
	playbackConfiguration := configuration
	playbackConfiguration.baseURL = configuration.playbackBaseURL
	if _, err := checker.client.readServerInfo(ctx, playbackConfiguration, "System/Info", true); err == nil {
		health.Status = integration.StatusHealthy
		health.Detail = "Emby 播放入口在线"
		return health
	} else if errors.Is(err, ErrUnauthorized) {
		health.Status = integration.StatusDegraded
		health.Detail = "播放入口可达，但 Emby 鉴权失败"
		return health
	}
	health.Status = integration.StatusUnavailable
	health.Detail = "无法连接 Emby 播放入口"
	return health
}

var _ integration.Checker = (*PlaybackChecker)(nil)
