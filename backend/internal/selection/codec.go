package selection

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"
)

var ErrInvalidToken = errors.New("invalid or expired selection token")

var (
	selectionAssociatedData = []byte("media-hub-selection-v1")
	providerAssociatedData  = []byte("media-hub-provider-result-v1")
)

type Payload struct {
	SourceID     string `json:"sourceId"`
	CandidateID  string `json:"candidateId"`
	Title        string `json:"title"`
	Year         int    `json:"year"`
	Season       int    `json:"season,omitempty"`
	EpisodeStart int    `json:"episodeStart,omitempty"`
	EpisodeEnd   int    `json:"episodeEnd,omitempty"`
	MediaType    string `json:"mediaType"`
	TMDBID       string `json:"tmdbId"`
	Reference    string `json:"reference"`
	ExpiresAt    int64  `json:"expiresAt"`
}

type ProviderPayload struct {
	OperationID string `json:"operationId"`
	FileID      string `json:"fileId"`
	Path        string `json:"path"`
	IsFile      bool   `json:"isFile"`
}

type Codec struct {
	aead cipher.AEAD
	now  func() time.Time
}

func NewCodec(encodedKey string) (*Codec, error) {
	if encodedKey == "" {
		return nil, nil
	}
	key, err := decodeKey(encodedKey)
	if err != nil || len(key) != 32 {
		return nil, errors.New("MEDIA_HUB_DATA_ENCRYPTION_KEY must be a base64-encoded 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("initialize selection cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("initialize selection cipher mode: %w", err)
	}
	return &Codec{aead: aead, now: time.Now}, nil
}

func (c *Codec) Encode(payload Payload) (string, error) {
	if c == nil {
		return "", ErrInvalidToken
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode selection payload: %w", err)
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate selection nonce: %w", err)
	}
	sealed := c.aead.Seal(nonce, nonce, encoded, selectionAssociatedData)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (c *Codec) Decode(token string) (Payload, error) {
	if c == nil {
		return Payload{}, ErrInvalidToken
	}
	sealed, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(sealed) <= c.aead.NonceSize() {
		return Payload{}, ErrInvalidToken
	}
	nonce := sealed[:c.aead.NonceSize()]
	plaintext, err := c.aead.Open(nil, nonce, sealed[c.aead.NonceSize():], selectionAssociatedData)
	if err != nil {
		return Payload{}, ErrInvalidToken
	}
	var payload Payload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return Payload{}, ErrInvalidToken
	}
	if payload.SourceID == "" || payload.CandidateID == "" || payload.Reference == "" ||
		!validTMDBID(payload.TMDBID) ||
		(payload.MediaType != "movie" && payload.MediaType != "series") ||
		payload.Season < 0 || payload.Season > 100 ||
		!validEpisodeRange(payload.EpisodeStart, payload.EpisodeEnd) ||
		(payload.MediaType == "movie" && (payload.Season != 0 || payload.EpisodeStart != 0 || payload.EpisodeEnd != 0)) ||
		(payload.MediaType == "series" && (payload.Season == 0 || payload.EpisodeStart == 0 || payload.EpisodeEnd == 0)) ||
		!c.now().UTC().Before(time.Unix(payload.ExpiresAt, 0).UTC()) {
		return Payload{}, ErrInvalidToken
	}
	return payload, nil
}

func (c *Codec) EncodeProvider(payload ProviderPayload) (string, error) {
	if c == nil || !validProviderPayload(payload) {
		return "", ErrInvalidToken
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode provider payload: %w", err)
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate provider nonce: %w", err)
	}
	sealed := c.aead.Seal(nonce, nonce, encoded, providerAssociatedData)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (c *Codec) DecodeProvider(token string) (ProviderPayload, error) {
	if c == nil {
		return ProviderPayload{}, ErrInvalidToken
	}
	sealed, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(sealed) <= c.aead.NonceSize() {
		return ProviderPayload{}, ErrInvalidToken
	}
	nonce := sealed[:c.aead.NonceSize()]
	plaintext, err := c.aead.Open(nil, nonce, sealed[c.aead.NonceSize():], providerAssociatedData)
	if err != nil {
		return ProviderPayload{}, ErrInvalidToken
	}
	var payload ProviderPayload
	if json.Unmarshal(plaintext, &payload) != nil || !validProviderPayload(payload) {
		return ProviderPayload{}, ErrInvalidToken
	}
	return payload, nil
}

func validProviderPayload(payload ProviderPayload) bool {
	if len(payload.OperationID) > 200 || len(payload.FileID) > 200 || len(payload.Path) > 4096 {
		return false
	}
	if payload.FileID != "" || payload.Path != "" {
		return payload.FileID != "" && payload.Path != ""
	}
	return payload.OperationID != ""
}

func validTMDBID(value string) bool {
	id, err := strconv.ParseInt(value, 10, 64)
	return err == nil && id > 0
}

func decodeKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err == nil {
		return key, nil
	}
	return base64.RawStdEncoding.DecodeString(encoded)
}

func validEpisodeRange(start, end int) bool {
	if start == 0 || end == 0 {
		return start == 0 && end == 0
	}
	return start > 0 && end >= start && end <= 10000
}
