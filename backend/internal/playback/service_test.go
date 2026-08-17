package playback

import (
	"context"
	"errors"
	"testing"
)

type sourceStub struct {
	media SourceMedia
	err   error
	ua    string
}

func (s *sourceStub) Resolve(_ context.Context, _, _, userAgent string) (SourceMedia, error) {
	s.ua = userAgent
	return s.media, s.err
}

func TestCreateReturnsBoundedHTTPSDescriptor(t *testing.T) {
	source := &sourceStub{media: SourceMedia{URL: "https://cdn.example/video.mkv?token=short", Name: "Movie.mkv"}}
	service := NewService(source)
	value, err := service.Create(context.Background(), "10", "20")
	if err != nil {
		t.Fatal(err)
	}
	if value.StreamURL != source.media.URL || value.Title != "Movie.mkv" || value.UserAgent != PlayerUserAgent || source.ua != PlayerUserAgent {
		t.Fatalf("descriptor = %#v ua=%q", value, source.ua)
	}
}

func TestCreateRejectsInvalidIDsAndUnsafeURLs(t *testing.T) {
	for _, test := range []struct {
		name     string
		parentID string
		fileID   string
		media    SourceMedia
		want     error
	}{
		{"invalid id", "../10", "20", SourceMedia{}, ErrInvalidRequest},
		{"http url", "10", "20", SourceMedia{URL: "http://cdn.example/a.mkv", Name: "a.mkv"}, ErrUnavailable},
		{"credentials", "10", "20", SourceMedia{URL: "https://user:pass@cdn.example/a.mkv", Name: "a.mkv"}, ErrUnavailable},
		{"non video", "10", "20", SourceMedia{URL: "https://cdn.example/a.txt", Name: "a.txt"}, ErrNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(&sourceStub{media: test.media})
			_, err := service.Create(context.Background(), test.parentID, test.fileID)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}
