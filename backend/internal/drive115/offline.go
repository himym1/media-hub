package drive115

import (
	"context"
	"net/url"
	"strings"
)

func (c *Client) AddOfflineURLs(ctx context.Context, destinationID string, urls []string) error {
	if len(urls) == 0 {
		return &WriteError{Code: "invalid_request", Err: ErrUpstreamResponse}
	}
	if destinationID == "" {
		destinationID = "0"
	}
	values := url.Values{}
	values.Set("wp_path_id", destinationID)
	values.Set("urls", strings.Join(urls, "\n"))
	return c.executeForm(ctx, "https://proapi.115.com/open/offline/add_task_urls", values)
}

func (s *AuthService) AddOfflineURLs(ctx context.Context, destinationID string, urls []string) error {
	if _, err := s.Status(ctx); err != nil {
		return err
	}
	return s.drive.AddOfflineURLs(ctx, destinationID, urls)
}
