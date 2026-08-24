package strm

import (
	"context"
	"errors"
	"time"
)

const retryAttempts = 3

func Retry(ctx context.Context, fn func() error) error {
	var err error
	for attempt := 0; attempt < retryAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err = fn()
		if err == nil || errors.Is(err, ErrAuthExpired) || errors.Is(err, ErrInvalidRequest) || errors.Is(err, ErrPathUnwritable) {
			return err
		}
		if attempt == retryAttempts-1 {
			break
		}
		delay := 200 * time.Millisecond * time.Duration(1<<attempt)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return err
}
