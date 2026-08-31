package emby

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func decodeEmbyBody(response *http.Response, target any) error {
	if target == nil || response.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(target); err != nil {
		return fmt.Errorf("decode Emby response: %w", err)
	}
	return nil
}
