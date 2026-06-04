package worker

import (
	"context"
	"errors"
	"syscall"
	"testing"
)

func TestRetryableClassifiesTimeoutFailures(t *testing.T) {
	cases := []error{
		context.DeadlineExceeded,
		fakeNetError{timeout: true},
		errors.New("timeout while dialing"),
	}

	for _, err := range cases {
		if !retryable(err) {
			t.Fatalf("retryable(%q) = false, want true", err)
		}
	}
}

func TestRetryableClassifiesNetworkFailures(t *testing.T) {
	cases := []error{
		fakeNetError{temporary: true},
		syscall.ECONNRESET,
		syscall.ECONNREFUSED,
		errors.New("connection reset by peer"),
		errors.New("temporary DNS failure"),
	}

	for _, err := range cases {
		if !retryable(err) {
			t.Fatalf("retryable(%q) = false, want true", err)
		}
	}
}

func TestRetryableRejectsCertificateVerificationFailures(t *testing.T) {
	err := errors.New("certificate verification failed")
	if retryable(err) {
		t.Fatalf("retryable(%q) = true, want false", err)
	}
}

type fakeNetError struct {
	timeout   bool
	temporary bool
}

func (e fakeNetError) Error() string {
	return "fake network error"
}

func (e fakeNetError) Timeout() bool {
	return e.timeout
}

func (e fakeNetError) Temporary() bool {
	return e.temporary
}
