package drive115

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"media-hub/backend/internal/integration"
	"media-hub/backend/internal/securepayload"
	"media-hub/backend/internal/store"
)

var (
	ErrAuthorizationUnavailable = errors.New("115 device authorization is unavailable")
	ErrAuthorizationPending     = errors.New("115 device authorization is pending")
	ErrAuthorizationExpired     = errors.New("115 device authorization expired")
	ErrLocalUploadUnavailable   = errors.New("115 cookie session does not support local upload")
)

const (
	qrTokenURL  = "https://qrcodeapi.115.com/api/1.0/web/1.0/token"
	qrStatusURL = "https://qrcodeapi.115.com/get/status/"
	qrLoginURL  = "https://passportapi.115.com/app/1.0/web/1.0/login/qrcode"
)

type AuthService struct {
	store      *store.Store
	codec      *securepayload.Codec
	drive      *Client
	httpClient *http.Client
	sessionMu  sync.Mutex
	tokenURL   string
	statusURL  string
	loginURL   string
	now        func() time.Time
}

type DeviceAuthorization struct {
	ID        string    `json:"id"`
	QRCode    string    `json:"qrCode,omitempty"`
	State     string    `json:"state"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type authorizationChallenge struct {
	UID  string `json:"uid"`
	Time int64  `json:"time"`
	Sign string `json:"sign"`
}

type credentialPayload struct {
	Cookie string `json:"cookie"`
}

func NewAuthService(dataStore *store.Store, codec *securepayload.Codec, drive *Client, timeout time.Duration) *AuthService {
	return &AuthService{
		store: dataStore, codec: codec, drive: drive,
		httpClient: &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		tokenURL:   qrTokenURL, statusURL: qrStatusURL, loginURL: qrLoginURL,
		now: time.Now,
	}
}

func (s *AuthService) Check(ctx context.Context) integration.Health {
	health := integration.Health{ID: "115", Label: "115"}
	status, err := s.Status(ctx)
	switch {
	case err == nil && status.Authorized:
		health.Status = integration.StatusHealthy
		health.Detail = "扫码授权正常"
	case errors.Is(err, ErrNotConfigured):
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未扫码授权"
	case errors.Is(err, ErrUnauthorized):
		health.Status = integration.StatusDegraded
		health.Detail = "扫码授权已失效"
	default:
		health.Status = integration.StatusUnavailable
		health.Detail = "无法读取 115 授权状态"
	}
	return health
}

func (s *AuthService) Status(ctx context.Context) (Status, error) {
	if s == nil || s.drive == nil {
		return Status{}, ErrNotConfigured
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	if err := s.applyStoredSession(ctx); err != nil {
		return Status{}, err
	}
	return s.drive.Status(ctx)
}

func (s *AuthService) prepareSession(ctx context.Context) error {
	if s == nil || s.drive == nil {
		return ErrNotConfigured
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	return s.applyStoredSession(ctx)
}

func (s *AuthService) Configured() bool {
	return s != nil && s.store != nil && s.codec != nil && s.drive != nil
}

func (s *AuthService) Load(ctx context.Context, userID int64) error {
	if s == nil || s.codec == nil || s.drive == nil {
		return nil
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	err := s.loadUserSession(ctx, userID)
	if errors.Is(err, ErrNotConfigured) {
		return nil
	}
	return err
}

func (s *AuthService) applyStoredSession(ctx context.Context) error {
	if s.codec == nil || s.store == nil {
		if s.drive.session() == "" {
			return ErrNotConfigured
		}
		return nil
	}
	admin, exists, err := s.store.Admin(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotConfigured
	}
	return s.loadUserSession(ctx, admin.ID)
}

func (s *AuthService) loadUserSession(ctx context.Context, userID int64) error {
	sealed, exists, err := s.store.ProviderCredential(ctx, userID, "115")
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotConfigured
	}
	var credential credentialPayload
	if err := s.codec.Open(sealed, &credential); err != nil {
		return fmt.Errorf("decrypt 115 credential: %w", err)
	}
	if credential.Cookie == "" {
		return ErrNotConfigured
	}
	s.drive.SetSession(credential.Cookie)
	return nil
}

func (s *AuthService) Start(ctx context.Context, userID int64) (DeviceAuthorization, error) {
	if !s.Configured() {
		return DeviceAuthorization{}, ErrAuthorizationUnavailable
	}
	var response struct {
		State json.RawMessage `json:"state"`
		Data  struct {
			UID    string      `json:"uid"`
			Time   json.Number `json:"time"`
			QRCode string      `json:"qrcode"`
			Sign   string      `json:"sign"`
		} `json:"data"`
	}
	if err := s.getJSON(ctx, s.tokenURL, nil, &response); err != nil {
		return DeviceAuthorization{}, err
	}
	if !qrStateSuccessful(response.State) {
		return DeviceAuthorization{}, ErrUpstreamResponse
	}
	issuedAt, _ := response.Data.Time.Int64()
	qrCode := strings.TrimSpace(response.Data.QRCode)
	if response.Data.UID == "" || issuedAt == 0 || response.Data.Sign == "" {
		return DeviceAuthorization{}, ErrUpstreamResponse
	}
	if qrCode == "" {
		qrCode = "https://115.com/scan/dg-" + response.Data.UID
	}
	id, err := randomToken(18)
	if err != nil {
		return DeviceAuthorization{}, err
	}
	payloadToken, err := s.codec.Seal(authorizationChallenge{
		UID: response.Data.UID, Time: issuedAt, Sign: response.Data.Sign,
	})
	if err != nil {
		return DeviceAuthorization{}, err
	}
	now := s.now().UTC()
	expiresAt := now.Add(10 * time.Minute)
	if err := s.store.CreateProviderAuthChallenge(ctx, store.ProviderAuthChallenge{
		ID: id, UserID: userID, Provider: "115", PayloadToken: payloadToken, State: "pending",
		ExpiresAt: expiresAt.Unix(), CreatedAt: now.Unix(), UpdatedAt: now.Unix(),
	}); err != nil {
		return DeviceAuthorization{}, err
	}
	return DeviceAuthorization{ID: id, QRCode: qrCode, State: "pending", ExpiresAt: expiresAt}, nil
}

func (s *AuthService) Poll(ctx context.Context, userID int64, id string) (DeviceAuthorization, error) {
	if !s.Configured() {
		return DeviceAuthorization{}, ErrAuthorizationUnavailable
	}
	stored, err := s.store.ProviderAuthChallenge(ctx, userID, id, "115")
	if err != nil {
		return DeviceAuthorization{}, err
	}
	expiresAt := time.Unix(stored.ExpiresAt, 0).UTC()
	if stored.State != "pending" {
		return DeviceAuthorization{ID: id, State: stored.State, ExpiresAt: expiresAt}, nil
	}
	if !s.now().UTC().Before(expiresAt) {
		_ = s.store.SetProviderAuthChallengeState(ctx, userID, id, "115", "pending", "expired", s.now())
		return DeviceAuthorization{ID: id, State: "expired", ExpiresAt: expiresAt}, nil
	}
	var challenge authorizationChallenge
	if err := s.codec.Open(stored.PayloadToken, &challenge); err != nil {
		return DeviceAuthorization{}, err
	}
	var status struct {
		State json.RawMessage `json:"state"`
		Data  struct {
			Status int `json:"status"`
		} `json:"data"`
	}
	query := url.Values{"uid": {challenge.UID}, "time": {strconv.FormatInt(challenge.Time, 10)}, "sign": {challenge.Sign}}
	if err := s.getJSON(ctx, s.statusURL, query, &status); err != nil {
		return DeviceAuthorization{}, err
	}
	if qrStateExpired(status.State) || status.Data.Status == -1 || status.Data.Status == -2 {
		_ = s.store.SetProviderAuthChallengeState(ctx, userID, id, "115", "pending", "expired", s.now())
		return DeviceAuthorization{ID: id, State: "expired", ExpiresAt: expiresAt}, nil
	}
	if !qrStateSuccessful(status.State) {
		return DeviceAuthorization{}, ErrUpstreamResponse
	}
	if status.Data.Status != 2 {
		return DeviceAuthorization{ID: id, State: "pending", ExpiresAt: expiresAt}, nil
	}
	cookie, err := s.exchange(ctx, challenge)
	if err != nil {
		return DeviceAuthorization{}, err
	}
	sealed, err := s.codec.Seal(credentialPayload{Cookie: cookie})
	if err != nil {
		return DeviceAuthorization{}, err
	}
	now := s.now().UTC()
	if err := s.store.UpsertProviderCredential(ctx, userID, "115", sealed, now); err != nil {
		return DeviceAuthorization{}, err
	}
	if err := s.store.SetProviderAuthChallengeState(ctx, userID, id, "115", "pending", "confirmed", now); err != nil {
		return DeviceAuthorization{}, err
	}
	s.sessionMu.Lock()
	s.drive.SetSession(cookie)
	s.sessionMu.Unlock()
	return DeviceAuthorization{ID: id, State: "confirmed", ExpiresAt: expiresAt}, nil
}

func (s *AuthService) exchange(ctx context.Context, challenge authorizationChallenge) (string, error) {
	var response struct {
		State json.RawMessage `json:"state"`
		Data  struct {
			Cookie sessionCookie `json:"cookie"`
			UID    string        `json:"UID"`
			CID    string        `json:"CID"`
			SEID   string        `json:"SEID"`
			KID    string        `json:"KID"`
		} `json:"data"`
	}
	httpResponse, err := s.postForm(ctx, s.loginURL, url.Values{"account": {challenge.UID}, "app": {"web"}}, &response)
	if err != nil {
		return "", err
	}
	if !qrStateSuccessful(response.State) {
		return "", ErrUpstreamResponse
	}
	cookie := response.Data.Cookie.header()
	if cookie == "" {
		cookie = sessionCookie{UID: response.Data.UID, CID: response.Data.CID, SEID: response.Data.SEID, KID: response.Data.KID}.header()
	}
	if cookie == "" && httpResponse != nil {
		cookie = cookiesFromResponse(httpResponse)
	}
	if cookie == "" {
		return "", ErrUpstreamResponse
	}
	return cookie, nil
}

type sessionCookie struct {
	UID  string `json:"UID"`
	CID  string `json:"CID"`
	SEID string `json:"SEID"`
	KID  string `json:"KID"`
}

func (value sessionCookie) header() string {
	if value.UID == "" || value.CID == "" || value.SEID == "" {
		return ""
	}
	parts := []string{"UID=" + value.UID, "CID=" + value.CID, "SEID=" + value.SEID}
	if value.KID != "" {
		parts = append(parts, "KID="+value.KID)
	}
	return strings.Join(parts, "; ")
}

func cookiesFromResponse(response *http.Response) string {
	var uid, cid, seid, kid string
	for _, cookie := range response.Cookies() {
		switch strings.ToUpper(cookie.Name) {
		case "UID":
			uid = cookie.Value
		case "CID":
			cid = cookie.Value
		case "SEID":
			seid = cookie.Value
		case "KID":
			kid = cookie.Value
		}
	}
	return sessionCookie{UID: uid, CID: cid, SEID: seid, KID: kid}.header()
}

func qrStateExpired(raw json.RawMessage) bool {
	text := strings.TrimSpace(string(raw))
	return text == "0" || text == "false"
}

func qrStateSuccessful(raw json.RawMessage) bool {
	text := strings.TrimSpace(string(raw))
	return text == "1" || text == "true"
}

func (s *AuthService) postForm(ctx context.Context, endpoint string, form url.Values, target any) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return s.doJSON(request, target)
}

func (s *AuthService) getJSON(ctx context.Context, endpoint string, query url.Values, target any) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	if query != nil {
		parsed.RawQuery = query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return err
	}
	_, err = s.doJSON(request, target)
	return err
}

func (s *AuthService) doJSON(request *http.Request, target any) (*http.Response, error) {
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", userAgent)
	response, err := s.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request 115 authorization: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return response, ErrUpstreamResponse
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(target); err != nil {
		return response, ErrUpstreamResponse
	}
	return response, nil
}

func randomToken(bytes int) (string, error) {
	value := make([]byte, bytes)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (s *AuthService) ListFiles(ctx context.Context, parentID string, limit, offset int) ([]FileItem, int, error) {
	if err := s.prepareSession(ctx); err != nil {
		return nil, 0, err
	}
	return s.drive.ListFiles(ctx, parentID, limit, offset)
}

func (s *AuthService) FolderPath(ctx context.Context, folderID string) (string, error) {
	if err := s.prepareSession(ctx); err != nil {
		return "", err
	}
	return s.drive.FolderPath(ctx, folderID)
}

func (s *AuthService) ExecuteFileCommand(ctx context.Context, operation string, input map[string]any) error {
	if err := s.prepareSession(ctx); err != nil {
		return err
	}
	return s.drive.ExecuteFileCommand(ctx, operation, input)
}

func (s *AuthService) UploadLocalFile(ctx context.Context, userID int64, path, destinationID string, progress func(int64, int64)) error {
	return ErrLocalUploadUnavailable
}
