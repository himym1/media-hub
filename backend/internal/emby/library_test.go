package emby

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPrimaryImageIsAuthenticatedAndBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/Items/item-1/Images/Primary" || request.URL.Query().Get("maxWidth") != "320" || request.Header.Get("X-Emby-Token") != "emby-key" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte{0xff, 0xd8, 0xff, 0xd9})
	}))
	defer server.Close()

	client := NewClient(server.URL, "emby-key", time.Second)
	image, err := client.PrimaryImage(context.Background(), "item-1", 320)
	if err != nil {
		t.Fatal(err)
	}
	if image.ContentType != "image/jpeg" || len(image.Data) != 4 {
		t.Fatalf("image = %#v", image)
	}
}
