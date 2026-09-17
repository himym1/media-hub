package emby

import (
	"strings"
)

// extractMediaTechSpecs 从 Emby 媒体源解析音视频规格铭牌信息
func extractMediaTechSpecs(sources []mediaSource) *MediaTechSpecs {
	if len(sources) == 0 {
		return nil
	}
	// 首选包含 MediaStreams 的 source
	var activeSource *mediaSource
	for i := range sources {
		if len(sources[i].MediaStreams) > 0 {
			activeSource = &sources[i]
			break
		}
	}
	if activeSource == nil {
		c := strings.TrimSpace(sources[0].Container)
		if c != "" && !strings.EqualFold(c, "strm") {
			return &MediaTechSpecs{Container: strings.ToUpper(c)}
		}
		return nil
	}

	var videoStream *mediaStream
	var audioStreams []mediaStream
	for i := range activeSource.MediaStreams {
		stream := &activeSource.MediaStreams[i]
		switch stream.Type {
		case "Video":
			if videoStream == nil {
				videoStream = stream
			}
		case "Audio":
			audioStreams = append(audioStreams, *stream)
		}
	}

	specs := &MediaTechSpecs{}
	if activeSource.Container != "" && !strings.EqualFold(activeSource.Container, "strm") {
		specs.Container = strings.ToUpper(strings.TrimSpace(activeSource.Container))
	}

	if videoStream != nil {
		specs.Resolution = formatResolution(videoStream.Width, videoStream.Height, videoStream.DisplayTitle)
		specs.VideoRange = formatVideoRange(videoStream)
		specs.VideoCodec = formatVideoCodec(videoStream.Codec)
		specs.BitDepth = videoStream.BitDepth
		if videoStream.AspectRatio != "" {
			specs.AspectRatio = videoStream.AspectRatio
		}
	}

	if len(audioStreams) > 0 {
		selectedAudio := pickPreferredAudioStream(audioStreams)
		specs.AudioProfile = formatAudioProfile(selectedAudio)
		specs.AudioCodec = formatAudioCodec(selectedAudio.Codec, selectedAudio.Profile, selectedAudio.DisplayTitle)
		specs.AudioChannels, specs.AudioChannelCount = formatAudioChannels(selectedAudio)
	}

	if specs.Resolution == "" && specs.VideoCodec == "" && specs.VideoRange == "" &&
		specs.AudioCodec == "" && specs.AudioProfile == "" && specs.AudioChannels == "" &&
		specs.Container == "" {
		return nil
	}

	return specs
}

func formatResolution(width, height int, displayTitle string) string {
	if width >= 3800 || height >= 2100 {
		return "4K UHD"
	}
	if width >= 2500 || height >= 1400 {
		return "2K"
	}
	if width >= 1900 || height >= 1000 {
		return "1080p"
	}
	if width >= 1200 || height >= 700 {
		return "720p"
	}
	if width > 0 || height > 0 {
		return "SD"
	}
	dt := strings.ToLower(displayTitle)
	if strings.Contains(dt, "4k") || strings.Contains(dt, "2160") {
		return "4K UHD"
	}
	if strings.Contains(dt, "1080") {
		return "1080p"
	}
	if strings.Contains(dt, "720") {
		return "720p"
	}
	return ""
}

func formatVideoRange(s *mediaStream) string {
	vr := strings.ToUpper(s.VideoRange)
	vrt := strings.ToUpper(s.VideoRangeType)
	dt := strings.ToUpper(s.DisplayTitle)
	prof := strings.ToUpper(s.Profile)
	doviTitle := strings.TrimSpace(s.VideoDoViTitle)

	isDoVi := doviTitle != "" ||
		strings.Contains(vr, "DOVI") || strings.Contains(vrt, "DOVI") ||
		strings.Contains(dt, "DOVI") || strings.Contains(dt, "DOLBY VISION") ||
		strings.Contains(prof, "DOLBY VISION")

	isHdr10Plus := strings.Contains(vr, "HDR10+") || strings.Contains(vrt, "HDR10+") || strings.Contains(dt, "HDR10+")
	isHdr := strings.Contains(vr, "HDR") || strings.Contains(vrt, "HDR") || strings.Contains(dt, "HDR")

	if isDoVi {
		return "Dolby Vision"
	}
	if isHdr10Plus {
		return "HDR10+"
	}
	if isHdr {
		return "HDR10"
	}
	if vr == "SDR" || vrt == "SDR" {
		return "SDR"
	}
	if s.BitDepth >= 10 {
		return "10-Bit"
	}
	return ""
}

func formatVideoCodec(codec string) string {
	switch strings.ToLower(strings.TrimSpace(codec)) {
	case "hevc", "h265", "x265":
		return "HEVC"
	case "h264", "avc", "x264":
		return "H.264"
	case "av1":
		return "AV1"
	case "vp9":
		return "VP9"
	case "vc1":
		return "VC-1"
	case "mpeg4":
		return "MPEG-4"
	case "mpeg2video":
		return "MPEG-2"
	default:
		return strings.ToUpper(strings.TrimSpace(codec))
	}
}

func pickPreferredAudioStream(streams []mediaStream) mediaStream {
	var best mediaStream
	bestScore := -1

	for _, s := range streams {
		score := 0
		lowerProf := strings.ToLower(s.Profile)
		lowerDt := strings.ToLower(s.DisplayTitle)
		lowerSpatial := strings.ToLower(s.AudioSpatialFormat)

		if strings.Contains(lowerSpatial, "atmos") || strings.Contains(lowerProf, "atmos") || strings.Contains(lowerDt, "atmos") {
			score += 1000
		}
		if strings.Contains(lowerSpatial, "dts:x") || strings.Contains(lowerProf, "dts:x") || strings.Contains(lowerDt, "dts:x") {
			score += 900
		}
		if strings.EqualFold(s.Codec, "truehd") {
			score += 500
		} else if strings.Contains(strings.ToUpper(s.Profile), "DTS-HD") || strings.Contains(strings.ToUpper(s.DisplayTitle), "DTS-HD") {
			score += 400
		}
		if s.IsDefault {
			score += 50
		}
		score += s.Channels * 10
		if score > bestScore {
			bestScore = score
			best = s
		}
	}
	return best
}

func formatAudioProfile(s mediaStream) string {
	lowerSpatial := strings.ToLower(s.AudioSpatialFormat)
	lowerProf := strings.ToLower(s.Profile)
	lowerDt := strings.ToLower(s.DisplayTitle)
	if strings.Contains(lowerSpatial, "atmos") || strings.Contains(lowerProf, "atmos") || strings.Contains(lowerDt, "atmos") {
		return "Dolby Atmos"
	}
	if strings.Contains(lowerSpatial, "dts:x") || strings.Contains(lowerProf, "dts:x") || strings.Contains(lowerDt, "dts:x") {
		return "DTS:X"
	}
	return ""
}

func formatAudioCodec(codec, profile, displayTitle string) string {
	c := strings.ToLower(strings.TrimSpace(codec))
	profUpper := strings.ToUpper(profile)
	dtUpper := strings.ToUpper(displayTitle)

	switch c {
	case "truehd":
		return "TrueHD"
	case "dts":
		if strings.Contains(profUpper, "DTS-HD MA") || strings.Contains(dtUpper, "DTS-HD MA") {
			return "DTS-HD MA"
		}
		if strings.Contains(profUpper, "DTS-HD") || strings.Contains(dtUpper, "DTS-HD") {
			return "DTS-HD"
		}
		return "DTS"
	case "eac3":
		return "E-AC-3"
	case "ac3":
		return "AC-3"
	case "aac":
		return "AAC"
	case "flac":
		return "FLAC"
	case "alac":
		return "ALAC"
	case "opus":
		return "Opus"
	case "pcm", "pcm_s16le", "pcm_s24le":
		return "PCM"
	default:
		return strings.ToUpper(c)
	}
}

func formatAudioChannels(s mediaStream) (string, int) {
	channels := s.Channels
	layout := strings.ToLower(s.ChannelLayout)

	if strings.Contains(layout, "7.1") {
		return "7.1", 8
	}
	if strings.Contains(layout, "5.1") {
		return "5.1", 6
	}
	if strings.Contains(layout, "stereo") || layout == "2.0" {
		return "2.0", 2
	}
	if strings.Contains(layout, "mono") || layout == "1.0" {
		return "1.0", 1
	}

	switch channels {
	case 8:
		return "7.1", 8
	case 7:
		return "6.1", 7
	case 6:
		return "5.1", 6
	case 2:
		return "2.0", 2
	case 1:
		return "1.0", 1
	default:
		if s.ChannelLayout != "" {
			return s.ChannelLayout, channels
		}
		return "", channels
	}
}
