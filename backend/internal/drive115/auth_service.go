package drive115

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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
)

const (
	deviceCodeEndpoint = "https://passportapi.115.com/open/authDeviceCode"
	deviceStatusURL    = "https://qrcodeapi.115.com/get/status/"
	deviceTokenURL     = "https://passportapi.115.com/open/deviceCodeToToken"
	refreshTokenURL    = "https://passportapi.115.com/open/refreshToken"
)

type AuthService struct {
	store              *store.Store
	codec              *securepayload.Codec
	clientIDMu         sync.RWMutex
	clientID           string
	drive              *Client
	httpClient         *http.Client
	refreshMu          sync.Mutex
	deviceCodeEndpoint string
	deviceStatusURL    string
	deviceTokenURL     string
	refreshTokenURL    string
	now                func() time.Time
}

type DeviceAuthorization struct {
	ID        string    `json:"id"`
	QRCode    string    `json:"qrCode,omitempty"`
	State     string    `json:"state"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type authorizationChallenge struct {
	UID      string `json:"uid"`
	Time     int64  `json:"time"`
	Sign     string `json:"sign"`
	Verifier string `json:"verifier"`
}

type credentialPayload struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    int64  `json:"expiresAt"`
}

func NewAuthService(dataStore *store.Store, codec *securepayload.Codec, clientID string, drive *Client, timeout time.Duration) *AuthService {
	return &AuthService{
		store: dataStore, codec: codec, clientID: strings.TrimSpace(clientID), drive: drive,
		httpClient:         &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		deviceCodeEndpoint: deviceCodeEndpoint, deviceStatusURL: deviceStatusURL,
		deviceTokenURL: deviceTokenURL, refreshTokenURL: refreshTokenURL,
		now: time.Now,
	}
}

func (s *AuthService) Check(ctx context.Context) integration.Health {
	health := integration.Health{ID: "115", Label: "115"}
	status, err := s.Status(ctx)
	switch {
	case err == nil && status.Authorized:
		health.Status = integration.StatusHealthy
		health.Detail = "开放平台授权正常"
	case errors.Is(err, ErrNotConfigured):
		health.Status = integration.StatusUnconfigured
		health.Detail = "尚未配置开放平台授权"
	case errors.Is(err, ErrUnauthorized):
		health.Status = integration.StatusDegraded
		health.Detail = "开放平台授权已失效"
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
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	if s.codec == nil || s.store == nil {
		return s.drive.Status(ctx)
	}
	admin, exists, err := s.store.Admin(ctx)
	if err != nil || !exists {
		return Status{}, err
	}
	sealed, exists, err := s.store.ProviderCredential(ctx, admin.ID, "115")
	if err != nil || !exists {
		return s.drive.Status(ctx)
	}
	var credential credentialPayload
	if err := s.codec.Open(sealed, &credential); err != nil {
		return Status{}, err
	}
	if credential.RefreshToken != "" && s.now().UTC().Unix() >= credential.ExpiresAt-300 {
		credential, err = s.refresh(ctx, credential.RefreshToken)
		if err != nil {
			return Status{}, err
		}
		sealed, err = s.codec.Seal(credential)
		if err != nil {
			return Status{}, err
		}
		if err := s.store.UpsertProviderCredential(ctx, admin.ID, "115", sealed, s.now()); err != nil {
			return Status{}, err
		}
	}
	s.drive.SetAccessToken(credential.AccessToken)
	return s.drive.Status(ctx)
}

func (s *AuthService) ConfigureClientID(clientID string) {
	if s == nil {
		return
	}
	s.clientIDMu.Lock()
	s.clientID = strings.TrimSpace(clientID)
	s.clientIDMu.Unlock()
}

func (s *AuthService) configuredClientID() string {
	if s == nil {
		return ""
	}
	s.clientIDMu.RLock()
	defer s.clientIDMu.RUnlock()
	return s.clientID
}

func (s *AuthService) Configured() bool {
	return s != nil && s.store != nil && s.codec != nil && s.drive != nil && s.configuredClientID() != ""
}

func (s *AuthService) Load(ctx context.Context, userID int64) error {
	if s == nil || s.codec == nil || s.drive == nil {
		return nil
	}
	sealed, exists, err := s.store.ProviderCredential(ctx, userID, "115")
	if err != nil || !exists {
		return err
	}
	var credential credentialPayload
	if err := s.codec.Open(sealed, &credential); err != nil {
		return fmt.Errorf("decrypt 115 credential: %w", err)
	}
	if credential.AccessToken == "" {
		return ErrAuthorizationUnavailable
	}
	s.drive.SetAccessToken(credential.AccessToken)
	return nil
}

func (s *AuthService) Start(ctx context.Context, userID int64) (DeviceAuthorization, error) {
	if !s.Configured() {
		return DeviceAuthorization{}, ErrAuthorizationUnavailable
	}
	verifier, err := randomToken(48)
	if err != nil {
		return DeviceAuthorization{}, err
	}
	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])
	var response struct {
		State bool `json:"state"`
		Data  struct {
			UID    string `json:"uid"`
			Time   int64  `json:"time"`
			QRCode string `json:"qrcode"`
			Sign   string `json:"sign"`
		} `json:"data"`
	}
	clientID := s.configuredClientID()
	if err := s.formJSON(ctx, s.deviceCodeEndpoint, url.Values{
		"client_id": {clientID}, "code_challenge": {challenge}, "code_challenge_method": {"sha256"},
	}, &response); err != nil {
		return DeviceAuthorization{}, err
	}
	if response.Data.UID == "" || response.Data.Time == 0 || response.Data.Sign == "" || response.Data.QRCode == "" {
		return DeviceAuthorization{}, ErrUpstreamResponse
	}
	id, err := randomToken(18)
	if err != nil {
		return DeviceAuthorization{}, err
	}
	payloadToken, err := s.codec.Seal(authorizationChallenge{
		UID: response.Data.UID, Time: response.Data.Time, Sign: response.Data.Sign, Verifier: verifier,
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
	return DeviceAuthorization{ID: id, QRCode: response.Data.QRCode, State: "pending", ExpiresAt: expiresAt}, nil
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
		State int `json:"state"`
		Data  struct {
			Status int `json:"status"`
		} `json:"data"`
	}
	query := url.Values{"uid": {challenge.UID}, "time": {strconv.FormatInt(challenge.Time, 10)}, "sign": {challenge.Sign}}
	if err := s.getJSON(ctx, s.deviceStatusURL, query, &status); err != nil {
		return DeviceAuthorization{}, err
	}
	if status.State == 0 {
		_ = s.store.SetProviderAuthChallengeState(ctx, userID, id, "115", "pending", "expired", s.now())
		return DeviceAuthorization{ID: id, State: "expired", ExpiresAt: expiresAt}, nil
	}
	if status.Data.Status != 2 {
		return DeviceAuthorization{ID: id, State: "pending", ExpiresAt: expiresAt}, nil
	}
	credential, err := s.exchange(ctx, challenge)
	if err != nil {
		return DeviceAuthorization{}, err
	}
	sealed, err := s.codec.Seal(credential)
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
	s.drive.SetAccessToken(credential.AccessToken)
	return DeviceAuthorization{ID: id, State: "confirmed", ExpiresAt: expiresAt}, nil
}

func (s *AuthService) exchange(ctx context.Context, challenge authorizationChallenge) (credentialPayload, error) {
	var response struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Data         *struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int64  `json:"expires_in"`
		} `json:"data"`
	}
	if err := s.formJSON(ctx, s.deviceTokenURL, url.Values{
		"uid": {challenge.UID}, "code_verifier": {challenge.Verifier},
	}, &response); err != nil {
		return credentialPayload{}, err
	}
	if response.Data != nil && response.AccessToken == "" {
		response.AccessToken, response.RefreshToken, response.ExpiresIn = response.Data.AccessToken, response.Data.RefreshToken, response.Data.ExpiresIn
	}
	if response.AccessToken == "" || response.RefreshToken == "" || response.ExpiresIn < 1 {
		return credentialPayload{}, ErrUpstreamResponse
	}
	return credentialPayload{
		AccessToken: response.AccessToken, RefreshToken: response.RefreshToken,
		ExpiresAt: s.now().UTC().Add(time.Duration(response.ExpiresIn) * time.Second).Unix(),
	}, nil
}

func (s *AuthService) refresh(ctx context.Context, refreshToken string) (credentialPayload, error) {
	var response struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Data         *struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int64  `json:"expires_in"`
		} `json:"data"`
	}
	if err := s.formJSON(ctx, s.refreshTokenURL, url.Values{"refresh_token": {refreshToken}}, &response); err != nil {
		return credentialPayload{}, err
	}
	if response.Data != nil && response.AccessToken == "" {
		response.AccessToken, response.RefreshToken, response.ExpiresIn = response.Data.AccessToken, response.Data.RefreshToken, response.Data.ExpiresIn
	}
	if response.AccessToken == "" || response.ExpiresIn < 1 {
		return credentialPayload{}, ErrUpstreamResponse
	}
	if response.RefreshToken == "" {
		response.RefreshToken = refreshToken
	}
	return credentialPayload{
		AccessToken: response.AccessToken, RefreshToken: response.RefreshToken,
		ExpiresAt: s.now().UTC().Add(time.Duration(response.ExpiresIn) * time.Second).Unix(),
	}, nil
}

func (s *AuthService) formJSON(ctx context.Context, endpoint string, form url.Values, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return s.doJSON(request, target)
}

func (s *AuthService) getJSON(ctx context.Context, endpoint string, query url.Values, target any) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	parsed.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return err
	}
	return s.doJSON(request, target)
}

func (s *AuthService) doJSON(request *http.Request, target any) error {
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Media-Hub/115-open")
	response, err := s.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("request 115 authorization: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ErrUpstreamResponse
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(target); err != nil {
		return ErrUpstreamResponse
	}
	return nil
}

func randomToken(bytes int) (string, error) {
	value := make([]byte, bytes)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (s *AuthService) ListFiles(ctx context.Context, parentID string, limit, offset int) ([]FileItem, int, error) {
	if _, err := s.Status(ctx); err != nil {
		return nil, 0, err
	}
	return s.drive.ListFiles(ctx, parentID, limit, offset)
}

func (s *AuthService) ExecuteFileCommand(ctx context.Context, operation string, input map[string]any) error {
	if _, err := s.Status(ctx); err != nil {
		return err
	}
	return s.drive.ExecuteFileCommand(ctx, operation, input)
}

func (s *AuthService) UploadLocalFile(ctx context.Context, userID int64, path, destinationID string, progress func(int64, int64)) error {
	if s == nil || s.drive == nil {
		return ErrNotConfigured
	}
	s.refreshMu.Lock()
	credential := credentialPayload{}
	if s.codec != nil && s.store != nil {
		sealed, exists, err := s.store.ProviderCredential(ctx, userID, "115")
		if err != nil {
			s.refreshMu.Unlock()
			return err
		}
		if exists {
			if err := s.codec.Open(sealed, &credential); err != nil {
				s.refreshMu.Unlock()
				return fmt.Errorf("decrypt 115 credential: %w", err)
			}
			if credential.RefreshToken != "" && credential.ExpiresAt <= s.now().Add(time.Minute).Unix() {
				credential, err = s.refresh(ctx, credential.RefreshToken)
				if err != nil {
					s.refreshMu.Unlock()
					return err
				}
				sealed, err = s.codec.Seal(credential)
				if err != nil {
					s.refreshMu.Unlock()
					return err
				}
				if err := s.store.UpsertProviderCredential(ctx, userID, "115", sealed, s.now()); err != nil {
					s.refreshMu.Unlock()
					return err
				}
			}
		}
	}
	accessToken, refreshToken := credential.AccessToken, credential.RefreshToken
	if accessToken == "" {
		accessToken = s.drive.token()
	}
	s.refreshMu.Unlock()
	if accessToken == "" {
		return ErrNotConfigured
	}

	var refreshedAccess, refreshedRefresh string
	var refreshedMu sync.Mutex
	err := uploadLocalFile(ctx, accessToken, refreshToken, path, destinationID, UploadProgress(progress), func(access, refresh string) {
		refreshedMu.Lock()
		refreshedAccess, refreshedRefresh = access, refresh
		refreshedMu.Unlock()
	})
	refreshedMu.Lock()
	accessToken, refreshToken = refreshedAccess, refreshedRefresh
	refreshedMu.Unlock()
	if accessToken != "" && refreshToken != "" && s.codec != nil && s.store != nil {
		persistContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		credential = credentialPayload{AccessToken: accessToken, RefreshToken: refreshToken, ExpiresAt: s.now().Add(2 * time.Hour).Unix()}
		sealed, sealErr := s.codec.Seal(credential)
		if sealErr != nil && err == nil {
			return sealErr
		}
		if sealErr == nil {
			s.refreshMu.Lock()
			persistErr := s.store.UpsertProviderCredential(persistContext, userID, "115", sealed, s.now())
			s.refreshMu.Unlock()
			if persistErr != nil && err == nil {
				return persistErr
			}
			s.drive.SetAccessToken(accessToken)
		}
	}
	return err
}
