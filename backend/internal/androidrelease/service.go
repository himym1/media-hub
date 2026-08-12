package androidrelease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	ErrNotConfigured = errors.New("android release directory is not configured")
	ErrNotFound      = errors.New("android release was not found")
	ErrInvalid       = errors.New("android release metadata is invalid")
)

var (
	versionNamePattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
	sha256Pattern      = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type Release struct {
	VersionCode             int       `json:"versionCode"`
	VersionName             string    `json:"versionName"`
	MinimumSupportedVersion int       `json:"minimumSupportedVersionCode"`
	SHA256                  string    `json:"sha256"`
	SizeBytes               int64     `json:"sizeBytes"`
	PublishedAt             time.Time `json:"publishedAt"`
	Notes                   string    `json:"notes"`
	DownloadPath            string    `json:"downloadPath"`
}

type manifest struct {
	VersionCode             int       `json:"versionCode"`
	VersionName             string    `json:"versionName"`
	MinimumSupportedVersion int       `json:"minimumSupportedVersionCode"`
	SHA256                  string    `json:"sha256"`
	SizeBytes               int64     `json:"sizeBytes"`
	PublishedAt             time.Time `json:"publishedAt"`
	Notes                   string    `json:"notes"`
}

type Service struct {
	directory string
}

func NewService(directory string) *Service {
	return &Service{directory: directory}
}

func (s *Service) Latest(context.Context) (Release, error) {
	value, err := s.readManifest()
	if err != nil {
		return Release{}, err
	}
	fileInfo, err := os.Stat(s.apkPath(value.VersionCode))
	if errors.Is(err, os.ErrNotExist) {
		return Release{}, ErrNotFound
	}
	if err != nil {
		return Release{}, err
	}
	if !fileInfo.Mode().IsRegular() || fileInfo.Size() != value.SizeBytes {
		return Release{}, ErrInvalid
	}
	return publicRelease(value), nil
}

func (s *Service) OpenAPK(ctx context.Context, versionCode int) (Release, io.ReadSeekCloser, error) {
	value, err := s.readManifest()
	if err != nil {
		return Release{}, nil, err
	}
	if versionCode != value.VersionCode {
		return Release{}, nil, ErrNotFound
	}
	file, err := os.Open(s.apkPath(versionCode))
	if errors.Is(err, os.ErrNotExist) {
		return Release{}, nil, ErrNotFound
	}
	if err != nil {
		return Release{}, nil, err
	}
	valid, err := validateAPK(ctx, file, value)
	if err != nil || !valid {
		_ = file.Close()
		if err != nil {
			return Release{}, nil, err
		}
		return Release{}, nil, ErrInvalid
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		_ = file.Close()
		return Release{}, nil, err
	}
	return publicRelease(value), file, nil
}

func (s *Service) readManifest() (manifest, error) {
	if s.directory == "" {
		return manifest{}, ErrNotConfigured
	}
	file, err := os.Open(filepath.Join(s.directory, "latest.json"))
	if errors.Is(err, os.ErrNotExist) {
		return manifest{}, ErrNotFound
	}
	if err != nil {
		return manifest{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(io.LimitReader(file, 64*1024))
	decoder.DisallowUnknownFields()
	var value manifest
	if err := decoder.Decode(&value); err != nil {
		return manifest{}, fmt.Errorf("decode Android release manifest: %w", ErrInvalid)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return manifest{}, ErrInvalid
	}
	value.VersionName = strings.TrimSpace(value.VersionName)
	value.SHA256 = strings.ToLower(strings.TrimSpace(value.SHA256))
	value.Notes = strings.TrimSpace(value.Notes)
	if value.VersionCode <= 0 || !versionNamePattern.MatchString(value.VersionName) ||
		value.MinimumSupportedVersion < 0 || value.MinimumSupportedVersion > value.VersionCode ||
		!sha256Pattern.MatchString(value.SHA256) || value.SizeBytes <= 0 || value.PublishedAt.IsZero() ||
		len(value.Notes) > 4000 {
		return manifest{}, ErrInvalid
	}
	return value, nil
}

func (s *Service) apkPath(versionCode int) string {
	return filepath.Join(s.directory, fmt.Sprintf("media-hub-%d.apk", versionCode))
}

func publicRelease(value manifest) Release {
	return Release{
		VersionCode:             value.VersionCode,
		VersionName:             value.VersionName,
		MinimumSupportedVersion: value.MinimumSupportedVersion,
		SHA256:                  value.SHA256,
		SizeBytes:               value.SizeBytes,
		PublishedAt:             value.PublishedAt.UTC(),
		Notes:                   value.Notes,
		DownloadPath:            fmt.Sprintf("/api/v1/client/android/releases/%d/apk", value.VersionCode),
	}
}

func validateAPK(ctx context.Context, file *os.File, value manifest) (bool, error) {
	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() || info.Size() != value.SizeBytes {
		return false, nil
	}
	hash := sha256.New()
	buffer := make([]byte, 64*1024)
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		count, readErr := file.Read(buffer)
		if count > 0 {
			_, _ = hash.Write(buffer[:count])
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return false, readErr
		}
	}
	return hex.EncodeToString(hash.Sum(nil)) == value.SHA256, nil
}
