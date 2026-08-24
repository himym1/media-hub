package strm

import (
	"context"
	"errors"
	"testing"
)

func TestRetryDoesNotRetryAuthOrUnwritable(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), func() error {
		calls++
		return ErrAuthExpired
	})
	if err != ErrAuthExpired || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	calls = 0
	err = Retry(context.Background(), func() error {
		calls++
		return ErrPathUnwritable
	})
	if !errors.Is(err, ErrPathUnwritable) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestRetryRepeatsTransientErrors(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), func() error {
		calls++
		return ErrListFailed
	})
	if err != ErrListFailed || calls != 3 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
