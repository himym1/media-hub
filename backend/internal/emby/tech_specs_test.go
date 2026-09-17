package emby

import (
	"testing"
)

func TestExtractMediaTechSpecs_DolbyVisionAtmos(t *testing.T) {
	sources := []mediaSource{
		{
			ID:        "src-1",
			Container: "mkv",
			MediaStreams: []mediaStream{
				{
					Type:           "Video",
					Codec:          "hevc",
					Width:          3840,
					Height:         2160,
					AspectRatio:    "2.39:1",
					BitDepth:       10,
					VideoRange:     "DOVIWithHDR10",
					VideoRangeType: "DOVI",
					VideoDoViTitle: "Dolby Vision Profile 7",
				},
				{
					Type:               "Audio",
					Codec:              "truehd",
					Profile:            "Dolby Atmos",
					AudioSpatialFormat: "DolbyAtmos",
					Channels:           8,
					ChannelLayout:      "7.1",
					IsDefault:          true,
				},
			},
		},
	}

	specs := extractMediaTechSpecs(sources)
	if specs == nil {
		t.Fatalf("expected non-nil specs")
	}
	if specs.Resolution != "4K UHD" {
		t.Errorf("expected 4K UHD, got %s", specs.Resolution)
	}
	if specs.VideoRange != "Dolby Vision" {
		t.Errorf("expected Dolby Vision, got %s", specs.VideoRange)
	}
	if specs.VideoCodec != "HEVC" {
		t.Errorf("expected HEVC, got %s", specs.VideoCodec)
	}
	if specs.BitDepth != 10 {
		t.Errorf("expected 10, got %d", specs.BitDepth)
	}
	if specs.AspectRatio != "2.39:1" {
		t.Errorf("expected 2.39:1, got %s", specs.AspectRatio)
	}
	if specs.AudioProfile != "Dolby Atmos" {
		t.Errorf("expected Dolby Atmos, got %s", specs.AudioProfile)
	}
	if specs.AudioCodec != "TrueHD" {
		t.Errorf("expected TrueHD, got %s", specs.AudioCodec)
	}
	if specs.AudioChannels != "7.1" {
		t.Errorf("expected 7.1, got %s", specs.AudioChannels)
	}
	if specs.AudioChannelCount != 8 {
		t.Errorf("expected 8, got %d", specs.AudioChannelCount)
	}
	if specs.Container != "MKV" {
		t.Errorf("expected MKV, got %s", specs.Container)
	}
}

func TestExtractMediaTechSpecs_Hdr10DtsHd(t *testing.T) {
	sources := []mediaSource{
		{
			ID:        "src-2",
			Container: "mp4",
			MediaStreams: []mediaStream{
				{
					Type:           "Video",
					Codec:          "h264",
					Width:          1920,
					Height:         1080,
					VideoRange:     "HDR",
					VideoRangeType: "HDR10",
				},
				{
					Type:          "Audio",
					Codec:         "dts",
					Profile:       "DTS-HD MA",
					Channels:      6,
					ChannelLayout: "5.1",
				},
			},
		},
	}

	specs := extractMediaTechSpecs(sources)
	if specs == nil {
		t.Fatalf("expected non-nil specs")
	}
	if specs.Resolution != "1080p" {
		t.Errorf("expected 1080p, got %s", specs.Resolution)
	}
	if specs.VideoRange != "HDR10" {
		t.Errorf("expected HDR10, got %s", specs.VideoRange)
	}
	if specs.VideoCodec != "H.264" {
		t.Errorf("expected H.264, got %s", specs.VideoCodec)
	}
	if specs.AudioCodec != "DTS-HD MA" {
		t.Errorf("expected DTS-HD MA, got %s", specs.AudioCodec)
	}
	if specs.AudioChannels != "5.1" {
		t.Errorf("expected 5.1, got %s", specs.AudioChannels)
	}
}

func TestExtractMediaTechSpecs_Empty(t *testing.T) {
	if specs := extractMediaTechSpecs(nil); specs != nil {
		t.Errorf("expected nil for empty sources")
	}
	if specs := extractMediaTechSpecs([]mediaSource{}); specs != nil {
		t.Errorf("expected nil for empty slice")
	}
}
