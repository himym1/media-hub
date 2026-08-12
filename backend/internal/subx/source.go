package subx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"media-hub/backend/internal/search"
)

const maxSourceReferenceBytes = 16 << 10

var (
	resolutionPattern = regexp.MustCompile(`(?i)\b(2160p|1080p|720p|4k)\b`)
	codecPattern      = regexp.MustCompile(`(?i)\b(hevc|h[. ]?265|avc|h[. ]?264|av1)\b`)
)

type Source struct {
	client  *Client
	service *Service
}

type sourceReference struct {
	OperationID string          `json:"operationId"`
	Body        json.RawMessage `json:"body"`
}

func NewSource(client *Client, service *Service) *Source {
	return &Source{client: client, service: service}
}

func (s *Source) ID() string    { return "subx" }
func (s *Source) Label() string { return "SubX 资源源" }

func (s *Source) Search(ctx context.Context, query string) ([]search.Candidate, error) {
	if s == nil || s.client == nil || !s.client.Configured() {
		return nil, search.Failure{Code: "source_unconfigured", Message: "SubX 资源源未配置", Retryable: false}
	}
	raw, err := s.client.readInternal(ctx, "sources.search", Invocation{Query: map[string]string{"keyword": query}})
	if err != nil {
		return nil, sourceFailure(err)
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return nil, search.Failure{Code: "invalid_source_response", Message: "SubX 搜索响应无效", Retryable: false}
	}
	candidates := make([]search.Candidate, 0)
	seen := map[string]struct{}{}
	walkObjects(value, func(object map[string]any) {
		candidate, ok := candidateFromObject(object)
		if !ok {
			return
		}
		if _, duplicate := seen[candidate.ID]; duplicate {
			return
		}
		seen[candidate.ID] = struct{}{}
		candidates = append(candidates, candidate)
	})
	if len(candidates) == 0 {
		return nil, search.Failure{Code: "invalid_source_response", Message: "SubX 未返回可识别的结构化资源", Retryable: false}
	}
	return candidates, nil
}

func (s *Source) StartTransfer(ctx context.Context, request search.TransferRequest) (search.TransferResult, error) {
	var reference sourceReference
	if s == nil || s.service == nil || json.Unmarshal([]byte(request.Reference), &reference) != nil {
		return search.TransferResult{}, search.Failure{Code: "invalid_source_reference", Message: "SubX 资源引用无效", Retryable: false}
	}
	operation, ok := LookupOperation(reference.OperationID)
	if !ok || !operation.Command || !operation.Body || operation.Group != "sources" || !strings.HasSuffix(operation.ID, ".save") {
		return search.TransferResult{}, search.Failure{Code: "invalid_source_reference", Message: "SubX 转存操作无效", Retryable: false}
	}
	var body map[string]any
	if json.Unmarshal(reference.Body, &body) != nil || body == nil {
		return search.TransferResult{}, search.Failure{Code: "invalid_source_reference", Message: "SubX 转存参数无效", Retryable: false}
	}
	encoded, err := json.Marshal(body)
	if err != nil || len(encoded) > maxRequestBytes {
		return search.TransferResult{}, search.Failure{Code: "invalid_source_reference", Message: "SubX 转存参数无效", Retryable: false}
	}
	job, _, err := s.service.enqueueInternal(ctx, request.UserID, operation.ID, request.IdempotencyKey, Invocation{Body: encoded})
	if err != nil {
		return search.TransferResult{}, sourceFailure(err)
	}
	return search.TransferResult{OperationID: job.ID, Status: "pending"}, nil
}

func (s *Source) TransferStatus(ctx context.Context, userID int64, operationID string) (search.TransferResult, error) {
	if s == nil || s.service == nil {
		return search.TransferResult{}, search.Failure{Code: "source_unconfigured", Message: "SubX 资源源未配置", Retryable: false}
	}
	job, raw, err := s.service.commandResult(ctx, userID, operationID)
	if err != nil {
		return search.TransferResult{}, sourceFailure(err)
	}
	switch job.State {
	case "queued", "submitting":
		return search.TransferResult{OperationID: operationID, Status: "pending"}, nil
	case "failed":
		return search.TransferResult{}, search.Failure{Code: job.ErrorCode, Message: job.ErrorMessage, Retryable: job.Retryable}
	case "needs_attention":
		return search.TransferResult{}, search.Failure{Code: "source_submission_unknown", Message: "SubX 转存结果需要人工确认", Retryable: false}
	case "completed":
		result, ok := transferResult(raw)
		if !ok {
			return search.TransferResult{}, search.Failure{Code: "source_submission_unknown", Message: "SubX 已执行转存，但返回结果不完整", Retryable: false}
		}
		result.OperationID = operationID
		return result, nil
	default:
		return search.TransferResult{}, search.Failure{Code: "invalid_source_response", Message: "SubX 命令状态无效", Retryable: false}
	}
}

func candidateFromObject(object map[string]any) (search.Candidate, bool) {
	provider := providerID(stringValue(object, "source", "source_name", "sourceKey", "site", "provider"))
	if provider == "" {
		return search.Candidate{}, false
	}
	title := stringValue(object, "title", "name", "resource_title", "media_title")
	id := stringValue(object, "id", "resource_id", "post_id", "tid", "slug")
	mediaType := mediaTypeValue(stringValue(object, "media_type", "mediaType", "type"))
	year := intValue(object, "year", "release_year")
	season := intValue(object, "season", "season_number")
	episodeStart := intValue(object, "episode_start", "episodeStart", "episode", "episode_number")
	episodeEnd := intValue(object, "episode_end", "episodeEnd")
	if episodeStart > 0 && episodeEnd == 0 {
		episodeEnd = episodeStart
	}
	if title == "" || len([]rune(title)) > 300 || id == "" || len(id) > 200 || mediaType == "" || year < 0 || year > 2100 || season < 0 || season > 100 {
		return search.Candidate{}, false
	}
	if mediaType == "movie" && (season != 0 || episodeStart != 0 || episodeEnd != 0) {
		return search.Candidate{}, false
	}
	if mediaType == "series" && (season == 0 || episodeStart == 0 || episodeEnd < episodeStart || episodeEnd > 10000) {
		return search.Candidate{}, false
	}
	release := releaseFacts(object)
	if release.Resolution == "" || release.VideoCodec == "" || release.SizeBytes < 0 {
		return search.Candidate{}, false
	}
	body, err := json.Marshal(object)
	if err != nil || len(body) > maxSourceReferenceBytes {
		return search.Candidate{}, false
	}
	reference, err := json.Marshal(sourceReference{OperationID: provider + ".save", Body: body})
	if err != nil {
		return search.Candidate{}, false
	}
	digest := sha256.Sum256(body)
	tmdbID := stringValue(object, "tmdb_id", "tmdbId")
	if !digits(tmdbID) {
		tmdbID = ""
	}
	return search.Candidate{
		ID: provider + "-" + id + "-" + hex.EncodeToString(digest[:6]), Title: title, Year: year,
		Season: season, EpisodeStart: episodeStart, EpisodeEnd: episodeEnd, MediaType: mediaType,
		TMDBID: tmdbID, Provider: provider,
		Release: release, SourceRef: string(reference),
	}, true
}

func releaseFacts(object map[string]any) search.ReleaseFacts {
	release := object
	if nested, ok := value(object, "release", "release_info", "quality").(map[string]any); ok {
		release = nested
	}
	text := strings.Join([]string{
		stringValue(release, "resolution", "quality", "video_quality"),
		stringValue(release, "video_codec", "videoCodec", "codec"),
		stringValue(object, "release_name", "subtitle", "description"),
	}, " ")
	resolution := normalizedResolution(stringValue(release, "resolution", "video_quality"), text)
	codec := normalizedCodec(stringValue(release, "video_codec", "videoCodec", "codec"), text)
	return search.ReleaseFacts{
		Resolution: resolution, VideoCodec: codec,
		DynamicRange: stringValue(release, "dynamic_range", "dynamicRange", "hdr"),
		Audio:        stringValue(release, "audio", "audio_codec", "audioCodec"),
		SizeBytes:    sizeValue(release, "size_bytes", "sizeBytes", "size"),
	}
}

func transferResult(raw json.RawMessage) (search.TransferResult, bool) {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return search.TransferResult{}, false
	}
	var result search.TransferResult
	walkObjects(value, func(object map[string]any) {
		if result.FileID != "" && result.Path != "" {
			return
		}
		fileID := stringValue(object, "file_id", "fileId", "cid", "target_cid", "folder_id")
		path := stringValue(object, "path", "file_path", "target_path", "folder_path")
		if fileID == "" || path == "" {
			return
		}
		result = search.TransferResult{Status: "completed", FileID: fileID, Path: path, IsFile: boolValue(object, "is_file", "isFile")}
	})
	return result, result.FileID != "" && result.Path != ""
}

func walkObjects(value any, visit func(map[string]any)) {
	switch typed := value.(type) {
	case map[string]any:
		visit(typed)
		for _, child := range typed {
			walkObjects(child, visit)
		}
	case []any:
		for _, child := range typed {
			walkObjects(child, visit)
		}
	}
}

func value(object map[string]any, names ...string) any {
	for key, candidate := range object {
		normalized := normalizeKey(key)
		for _, name := range names {
			if normalized == normalizeKey(name) {
				return candidate
			}
		}
	}
	return nil
}

func stringValue(object map[string]any, names ...string) string {
	candidate := value(object, names...)
	switch typed := candidate.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
	}
	return ""
}

func intValue(object map[string]any, names ...string) int {
	raw := stringValue(object, names...)
	if raw != "" {
		parsed, _ := strconv.Atoi(raw)
		return parsed
	}
	if number, ok := value(object, names...).(float64); ok {
		return int(number)
	}
	return 0
}

func boolValue(object map[string]any, names ...string) bool {
	candidate := value(object, names...)
	if typed, ok := candidate.(bool); ok {
		return typed
	}
	parsed, _ := strconv.ParseBool(stringValue(object, names...))
	return parsed
}

func sizeValue(object map[string]any, names ...string) int64 {
	candidate := value(object, names...)
	switch typed := candidate.(type) {
	case float64:
		return int64(typed)
	case json.Number:
		value, _ := typed.Int64()
		return value
	case string:
		raw := strings.TrimSpace(strings.ToUpper(typed))
		for suffix, multiplier := range map[string]float64{"TB": 1 << 40, "GB": 1 << 30, "MB": 1 << 20, "KB": 1 << 10, "B": 1} {
			if strings.HasSuffix(raw, suffix) {
				number := strings.TrimSpace(strings.TrimSuffix(raw, suffix))
				parsed, err := strconv.ParseFloat(number, 64)
				if err == nil {
					return int64(parsed * multiplier)
				}
			}
		}
	}
	return 0
}

func normalizedResolution(explicit, text string) string {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		if strings.EqualFold(explicit, "4k") {
			return "2160p"
		}
		return explicit
	}
	matched := resolutionPattern.FindString(text)
	if strings.EqualFold(matched, "4k") {
		return "2160p"
	}
	return strings.ToLower(matched)
}

func normalizedCodec(explicit, text string) string {
	value := strings.TrimSpace(explicit)
	if value == "" {
		value = codecPattern.FindString(text)
	}
	normalized := strings.ToLower(strings.NewReplacer(".", "", " ", "").Replace(value))
	switch normalized {
	case "hevc", "h265":
		return "HEVC"
	case "avc", "h264":
		return "AVC"
	case "av1":
		return "AV1"
	default:
		return value
	}
}

func mediaTypeValue(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "movie", "film", "电影":
		return "movie"
	case "series", "tv", "tvshow", "电视剧", "剧集":
		return "series"
	default:
		return ""
	}
}

func providerID(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	aliases := map[string]string{
		"dian": "dian", "点点": "dian", "framehdr": "framehdr", "帧影": "framehdr",
		"gimy": "gimy", "观影": "guanying", "guanying": "guanying", "hdhive": "hdhive",
		"juying": "juying", "聚影": "juying", "mikan": "mikan", "蜜柑": "mikan", "sidhub": "sidhub",
	}
	return aliases[normalized]
}

func normalizeKey(value string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(value))
}

func sourceFailure(err error) search.Failure {
	switch {
	case errors.Is(err, ErrUnauthorized):
		return search.Failure{Code: "source_unauthorized", Message: "SubX 鉴权失败", Retryable: false}
	case errors.Is(err, ErrInvalidInvocation), errors.Is(err, ErrInvalidResponse), errors.Is(err, ErrUnknownOperation):
		return search.Failure{Code: "invalid_source_response", Message: "SubX 请求或响应无效", Retryable: false}
	case errors.Is(err, ErrNotConfigured):
		return search.Failure{Code: "source_unconfigured", Message: "SubX 资源源未配置", Retryable: false}
	default:
		return search.Failure{Code: "source_unavailable", Message: "SubX 资源源暂时不可用", Retryable: true}
	}
}

func digits(value string) bool {
	if value == "" {
		return true
	}
	if value[0] == '0' {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func (s *Source) String() string {
	return fmt.Sprintf("SubX source configured=%t", s != nil && s.client != nil && s.client.Configured())
}
