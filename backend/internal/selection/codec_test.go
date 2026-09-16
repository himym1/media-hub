package selection

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestCodecRoundTripAndExpiry(t *testing.T) {
	codec, err := NewCodec(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	codec.now = func() time.Time { return now }
	payload := Payload{
		SourceID: "frame", CandidateID: "candidate", Title: "Movie", Year: 2026, TMDBID: "123",
		MediaType: "movie", Reference: "private-reference", ExpiresAt: now.Add(time.Minute).Unix(), Revision: 1,
	}
	token, err := codec.Encode(payload)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(token, payload.Reference) {
		t.Fatal("selection token exposed its reference")
	}
	decoded, err := codec.Decode(token)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != payload {
		t.Fatalf("decoded payload = %#v", decoded)
	}

	adult, err := codec.Encode(Payload{
		SourceID: "share", CandidateID: "share-1", Title: "SSIS-001", MediaType: "adult",
		Reference: "share-ref", ExpiresAt: now.Add(time.Minute).Unix(), Revision: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if decodedAdult, decodeErr := codec.Decode(adult); decodeErr != nil || decodedAdult.MediaType != "adult" || decodedAdult.TMDBID != "" {
		t.Fatalf("adult payload=%#v err=%v", decodedAdult, decodeErr)
	}

	codec.now = func() time.Time { return now.Add(2 * time.Minute) }
	if _, err := codec.Decode(token); err != ErrInvalidToken {
		t.Fatalf("expired token error = %v", err)
	}
}

func TestProviderPayloadIsEncryptedAtRest(t *testing.T) {
	codec, err := NewCodec(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatal(err)
	}
	payload := ProviderPayload{
		OperationID: "operation-private", FileID: "file-private", Path: "/private/media/path", IsFile: false,
	}
	token, err := codec.EncodeProvider(payload)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(token, payload.OperationID) || strings.Contains(token, payload.Path) {
		t.Fatal("provider token exposed private data")
	}
	decoded, err := codec.DecodeProvider(token)
	if err != nil || decoded != payload {
		t.Fatalf("decoded payload=%#v err=%v", decoded, err)
	}
}
