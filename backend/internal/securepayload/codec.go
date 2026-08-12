package securepayload

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

var ErrInvalidPayload = errors.New("secure payload is invalid")

type Codec struct {
	aead cipher.AEAD
}

func New(key string) (*Codec, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, nil
	}
	decoded, err := decodeKey(key)
	if err != nil || len(decoded) != 32 {
		return nil, errors.New("data encryption key must be a base64-encoded 32-byte key")
	}
	block, err := aes.NewCipher(decoded)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Codec{aead: aead}, nil
}

func (c *Codec) Seal(value any) (string, error) {
	if c == nil || c.aead == nil {
		return "", ErrInvalidPayload
	}
	plain, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nil, nonce, plain, nil)
	encoded := append(nonce, sealed...)
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func (c *Codec) Open(token string, target any) error {
	if c == nil || c.aead == nil || strings.TrimSpace(token) == "" {
		return ErrInvalidPayload
	}
	encoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(encoded) <= c.aead.NonceSize() {
		return ErrInvalidPayload
	}
	plain, err := c.aead.Open(nil, encoded[:c.aead.NonceSize()], encoded[c.aead.NonceSize():], nil)
	if err != nil {
		return ErrInvalidPayload
	}
	if err := json.Unmarshal(plain, target); err != nil {
		return ErrInvalidPayload
	}
	return nil
}

func Digest(value any) ([]byte, error) {
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(canonical)
	return digest[:], nil
}

func decodeKey(encoded string) ([]byte, error) {
	encodings := []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding}
	var lastErr error
	for _, encoding := range encodings {
		value, err := encoding.DecodeString(encoded)
		if err == nil {
			return value, nil
		}
		lastErr = err
	}
	return nil, lastErr
}
