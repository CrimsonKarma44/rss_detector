package rssdetector

import (
	"errors"
	"testing"
	"time"
)

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{"rate limit", &RateLimitError{StatusCode: 429, URL: "https://x", Attempt: 3}, ErrRateLimited},
		{"blocked", &BlockedError{StatusCode: 403, URL: "https://x", Reason: "captcha"}, ErrBlocked},
		{"fetch", &FetchError{StatusCode: 500, URL: "https://x"}, ErrFetchFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.err, tt.want) {
				t.Fatalf("errors.Is(%v, %v) = false", tt.err, tt.want)
			}
			if tt.err.Error() == "" {
				t.Fatal("empty Error()")
			}
		})
	}
}

func TestRateLimitErrorRetryAfter(t *testing.T) {
	e := &RateLimitError{
		StatusCode: 429,
		URL:        "https://example.com",
		Attempt:    2,
		RetryAfter: 3 * time.Second,
	}
	if !errors.Is(e, ErrRateLimited) {
		t.Fatal("expected ErrRateLimited")
	}
	if StatusFromError(e) != 429 {
		t.Fatalf("status = %d", StatusFromError(e))
	}
	msg := e.Error()
	if msg == "" {
		t.Fatal("empty message")
	}
}

func TestIsHTTPStatus(t *testing.T) {
	err := &FetchError{StatusCode: 404, URL: "https://x"}
	if !IsHTTPStatus(err, 404) {
		t.Fatal("expected 404 match")
	}
	if IsHTTPStatus(err, 500) {
		t.Fatal("unexpected 500 match")
	}
	if IsHTTPStatus(ErrNoFeeds, 404) {
		t.Fatal("sentinel should not match status")
	}
}
