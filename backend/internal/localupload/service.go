package localupload

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"media-hub/backend/internal/securepayload"
	"media-hub/backend/internal/store"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrUnavailable   = errors.New("local upload unavailable")
	ErrInvalidPath   = errors.New("invalid local upload path")
	ErrSourceChanged = errors.New("local upload source changed")
)

type Uploader interface {
	UploadLocalFile(context.Context, int64, string, string, func(int64, int64)) error
}
type Service struct {
	store *store.Store
	codec *securepayload.Codec
	roots []string
	now   func() time.Time
}
type Root struct {
	ID string `json:"id"`
}
type Entry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Directory bool   `json:"directory"`
	Size      int64  `json:"size"`
}
type Job struct {
	ID           string                   `json:"id"`
	Path         string                   `json:"path"`
	State        string                   `json:"state"`
	BytesDone    int64                    `json:"bytesDone"`
	BytesTotal   int64                    `json:"bytesTotal"`
	ErrorCode    string                   `json:"errorCode,omitempty"`
	ErrorMessage string                   `json:"errorMessage,omitempty"`
	CreatedAt    time.Time                `json:"createdAt"`
	UpdatedAt    time.Time                `json:"updatedAt"`
	Events       []store.LocalUploadEvent `json:"events,omitempty"`
}
type jobPayload struct {
	Root          int    `json:"root"`
	Path          string `json:"path"`
	DestinationID string `json:"destinationId"`
	Size          int64  `json:"size"`
	ModifiedNanos int64  `json:"modifiedNanos"`
}

func NewService(storage *store.Store, codec *securepayload.Codec, roots []string) *Service {
	return &Service{store: storage, codec: codec, roots: append([]string{}, roots...), now: time.Now}
}
func (s *Service) Roots() []Root {
	values := make([]Root, len(s.roots))
	for index := range s.roots {
		values[index] = Root{ID: rootID(index)}
	}
	return values
}
func (s *Service) List(rootIDValue, relative string) ([]Entry, error) {
	_, directory, err := s.resolve(rootIDValue, relative)
	if err != nil {
		return nil, err
	}
	items, err := os.ReadDir(directory)
	if err != nil {
		return nil, ErrInvalidPath
	}
	if len(items) > 500 {
		items = items[:500]
	}
	values := make([]Entry, 0, len(items))
	for _, item := range items {
		if item.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, err := item.Info()
		if err != nil || (!info.IsDir() && !info.Mode().IsRegular()) {
			continue
		}
		path := filepath.ToSlash(filepath.Join(filepath.Clean(relative), item.Name()))
		if path == "." {
			path = item.Name()
		}
		values = append(values, Entry{Name: item.Name(), Path: path, Directory: info.IsDir(), Size: info.Size()})
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].Directory != values[j].Directory {
			return values[i].Directory
		}
		return strings.ToLower(values[i].Name) < strings.ToLower(values[j].Name)
	})
	return values, nil
}
func (s *Service) Create(ctx context.Context, userID int64, key, rootIDValue, relative, destination string) (Job, error) {
	if s == nil || s.codec == nil || len(s.roots) == 0 {
		return Job{}, ErrUnavailable
	}
	rootIndex, path, err := s.resolve(rootIDValue, relative)
	if err != nil {
		return Job{}, err
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return Job{}, ErrInvalidPath
	}
	if !numeric(destination) || key == "" || len(key) > 200 {
		return Job{}, ErrInvalidPath
	}
	payload := jobPayload{Root: rootIndex, Path: filepath.ToSlash(filepath.Clean(relative)), DestinationID: destination, Size: info.Size(), ModifiedNanos: info.ModTime().UnixNano()}
	encoded, _ := json.Marshal(payload)
	token, err := s.codec.Seal(payload)
	if err != nil {
		return Job{}, err
	}
	digest := sha256.Sum256(encoded)
	id, err := newID()
	if err != nil {
		return Job{}, err
	}
	now := s.now().UTC()
	stored, _, err := s.store.CreateLocalUpload(ctx, store.LocalUploadJob{ID: id, UserID: userID, IdempotencyKey: key, RequestHash: digest[:], PayloadToken: token, State: "queued", BytesTotal: info.Size(), CreatedAt: now.Unix(), UpdatedAt: now.Unix()})
	if err != nil {
		return Job{}, err
	}
	return s.public(stored), nil
}
func (s *Service) ListJobs(ctx context.Context, userID int64, limit int) ([]Job, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	items, err := s.store.ListLocalUploads(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	values := make([]Job, 0, len(items))
	for _, item := range items {
		values = append(values, s.public(item))
	}
	return values, nil
}
func (s *Service) Get(ctx context.Context, userID int64, id string) (Job, error) {
	item, err := s.store.LocalUpload(ctx, userID, id)
	if err != nil {
		return Job{}, err
	}
	value := s.public(item)
	value.Events, err = s.store.LocalUploadEvents(ctx, userID, id)
	return value, err
}
func (s *Service) Retry(ctx context.Context, userID int64, id, confirmation string) (Job, error) {
	item, err := s.store.RetryLocalUpload(ctx, userID, id, confirmation, s.now())
	if err != nil {
		return Job{}, err
	}
	return s.public(item), nil
}
func (s *Service) payload(item store.LocalUploadJob) (jobPayload, string, error) {
	var value jobPayload
	if err := s.codec.Open(item.PayloadToken, &value); err != nil {
		return jobPayload{}, "", err
	}
	if value.Root < 0 || value.Root >= len(s.roots) {
		return jobPayload{}, "", ErrInvalidPath
	}
	_, path, err := s.resolve(rootID(value.Root), value.Path)
	if err != nil {
		return jobPayload{}, "", err
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return jobPayload{}, "", ErrInvalidPath
	}
	if info.Size() != value.Size || info.ModTime().UnixNano() != value.ModifiedNanos {
		return jobPayload{}, "", ErrSourceChanged
	}
	return value, path, nil
}
func (s *Service) public(item store.LocalUploadJob) Job {
	value := Job{ID: item.ID, State: item.State, BytesDone: item.BytesDone, BytesTotal: item.BytesTotal, ErrorCode: item.ErrorCode, ErrorMessage: item.ErrorMessage, CreatedAt: time.Unix(item.CreatedAt, 0).UTC(), UpdatedAt: time.Unix(item.UpdatedAt, 0).UTC()}
	if payload, _, err := s.payload(item); err == nil {
		value.Path = payload.Path
	}
	return value
}
func (s *Service) resolve(id, relative string) (int, string, error) {
	if s == nil {
		return 0, "", ErrUnavailable
	}
	index, err := parseRootID(id)
	if err != nil || index >= len(s.roots) {
		return 0, "", ErrInvalidPath
	}
	relative = filepath.Clean(filepath.FromSlash(strings.TrimSpace(relative)))
	if filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return 0, "", ErrInvalidPath
	}
	root, err := filepath.EvalSymlinks(s.roots[index])
	if err != nil {
		return 0, "", ErrInvalidPath
	}
	if err := rejectSymlinkComponents(root, relative); err != nil {
		return 0, "", ErrInvalidPath
	}
	candidate := filepath.Join(root, relative)
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return 0, "", ErrInvalidPath
	}
	within, err := filepath.Rel(root, resolved)
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
		return 0, "", ErrInvalidPath
	}
	return index, resolved, nil
}
func rejectSymlinkComponents(root, relative string) error {
	if relative == "." {
		return nil
	}
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return ErrInvalidPath
		}
	}
	return nil
}

func rootID(index int) string { return "root-" + strconv.Itoa(index+1) }
func parseRootID(value string) (int, error) {
	if !strings.HasPrefix(value, "root-") {
		return 0, ErrInvalidPath
	}
	number, err := strconv.Atoi(strings.TrimPrefix(value, "root-"))
	if err != nil || number < 1 {
		return 0, ErrInvalidPath
	}
	return number - 1, nil
}
func numeric(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func newID() (string, error) {
	value := make([]byte, 18)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "upl_" + base64.RawURLEncoding.EncodeToString(value), nil
}
