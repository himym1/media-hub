package playback

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type driveResolverStub struct {
	media  SourceMedia
	err    error
	target Drive115Target
	ua     string
}

func (s *driveResolverStub) ResolveDrive115(_ context.Context, target Drive115Target, userAgent string) (SourceMedia, error) {
	s.target = target
	s.ua = userAgent
	return s.media, s.err
}

type embyResolverStub struct {
	media  SourceMedia
	err    error
	target EmbyItemTarget
	ua     string
}

func (s *embyResolverStub) ResolveEmbyItem(_ context.Context, target EmbyItemTarget, userAgent string) (SourceMedia, error) {
	s.target = target
	s.ua = userAgent
	return s.media, s.err
}

type sessionReporterStub struct {
	reference string
	event     SessionEvent
}

func (s *sessionReporterStub) ReportPlayback(_ context.Context, reference string, event SessionEvent) error {
	s.reference = reference
	s.event = event
	return nil
}

func TestCreateDrive115ReturnsBoundedHTTPSDescriptor(t *testing.T) {
	drive := &driveResolverStub{media: SourceMedia{URL: "https://cdn.example/video.mkv?token=short", Name: "Movie.mkv"}}
	service := NewService(drive, nil)
	value, err := service.CreateDrive115(context.Background(), Drive115Target{ParentID: "10", FileID: "20"})
	if err != nil {
		t.Fatal(err)
	}
	if value.StreamURL != drive.media.URL || value.Title != "Movie.mkv" || value.UserAgent != PlayerUserAgent || drive.ua != PlayerUserAgent {
		t.Fatalf("descriptor = %#v ua=%q", value, drive.ua)
	}
	if drive.target.ParentID != "10" || drive.target.FileID != "20" {
		t.Fatalf("target = %#v", drive.target)
	}
}

func TestCreateEmbyItemUsesTypedResolver(t *testing.T) {
	emby := &embyResolverStub{media: SourceMedia{URL: "https://cdn.example/movie", Name: "Movie"}}
	service := NewService(nil, emby)
	value, err := service.CreateEmbyItem(context.Background(), 1, EmbyItemTarget{ItemID: "emby-item_20"})
	if err != nil {
		t.Fatal(err)
	}
	if value.Title != "Movie" || emby.target.ItemID != "emby-item_20" || emby.ua != PlayerUserAgent {
		t.Fatalf("descriptor=%#v target=%#v ua=%q", value, emby.target, emby.ua)
	}
}

func TestCreateRejectsInvalidTargetsAndUnsafeMedia(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(*Service) error
		want error
	}{
		{"invalid drive id", func(service *Service) error {
			_, err := service.CreateDrive115(context.Background(), Drive115Target{ParentID: "../10", FileID: "20"})
			return err
		}, ErrInvalidRequest},
		{"invalid emby id", func(service *Service) error {
			_, err := service.CreateEmbyItem(context.Background(), 1, EmbyItemTarget{ItemID: "bad.id"})
			return err
		}, ErrInvalidRequest},
		{"http url", func(service *Service) error {
			_, err := service.CreateEmbyItem(context.Background(), 1, EmbyItemTarget{ItemID: "item"})
			return err
		}, ErrUnavailable},
		{"non video drive file", func(service *Service) error {
			_, err := service.CreateDrive115(context.Background(), Drive115Target{ParentID: "10", FileID: "20"})
			return err
		}, ErrNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			drive := &driveResolverStub{media: SourceMedia{URL: "https://cdn.example/a.txt", Name: "a.txt"}}
			emby := &embyResolverStub{media: SourceMedia{URL: "http://cdn.example/a", Name: "Movie"}}
			err := test.run(NewService(drive, emby))
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestEmbyPlaybackSessionIsOpaqueUserBoundAndDeletedOnStop(t *testing.T) {
	reporter := &sessionReporterStub{}
	emby := &embyResolverStub{media: SourceMedia{
		URL: "https://cdn.example/movie", Name: "Movie",
		Session: &SourceSession{Reporter: reporter, Reference: "private-emby-reference"},
	}}
	service := NewService(nil, emby)
	descriptor, err := service.CreateEmbyItem(context.Background(), 7, EmbyItemTarget{ItemID: "item-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !validSessionID(descriptor.SessionID) || strings.Contains(descriptor.SessionID, "item") {
		t.Fatalf("session ID = %q", descriptor.SessionID)
	}
	if err := service.Report(context.Background(), 8, descriptor.SessionID, SessionEvent{Type: SessionStarted}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other user report error = %v", err)
	}
	if err := service.Report(context.Background(), 7, descriptor.SessionID, SessionEvent{Type: SessionStopped, PositionMS: 12_345}); err != nil {
		t.Fatal(err)
	}
	if reporter.reference != "private-emby-reference" || reporter.event.Type != SessionStopped || reporter.event.PositionMS != 12_345 {
		t.Fatalf("report = %#v reference=%q", reporter.event, reporter.reference)
	}
	if err := service.Report(context.Background(), 7, descriptor.SessionID, SessionEvent{Type: SessionProgress}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted session error = %v", err)
	}
}

func TestCreatingSessionRemovesExpiredEntries(t *testing.T) {
	reporter := &sessionReporterStub{}
	emby := &embyResolverStub{media: SourceMedia{
		URL: "https://cdn.example/movie", Name: "Movie",
		Session: &SourceSession{Reporter: reporter, Reference: "reference"},
	}}
	service := NewService(nil, emby)
	now := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	first, err := service.CreateEmbyItem(context.Background(), 1, EmbyItemTarget{ItemID: "item-1"})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(sessionTTL + time.Second)
	if _, err := service.CreateEmbyItem(context.Background(), 1, EmbyItemTarget{ItemID: "item-2"}); err != nil {
		t.Fatal(err)
	}
	service.mutex.Lock()
	_, firstExists := service.sessions[first.SessionID]
	count := len(service.sessions)
	service.mutex.Unlock()
	if firstExists || count != 1 {
		t.Fatalf("first exists=%v session count=%d", firstExists, count)
	}
}
