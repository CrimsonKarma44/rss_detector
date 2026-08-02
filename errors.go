package rssdetector

import (
	"errors"
	"fmt"
	"time"
)

// Sentinel errors for common failure modes.
var (
	ErrInvalidURL  = errors.New("rssdetector: invalid URL")
	ErrFetchFailed = errors.New("rssdetector: fetch failed")
	ErrRateLimited = errors.New("rssdetector: rate limited")
	ErrBlocked     = errors.New("rssdetector: blocked by bot protection")
	ErrNoFeeds     = errors.New("rssdetector: no feeds found")
	ErrNotHTML     = errors.New("rssdetector: content is not HTML")
)

// RateLimitError is returned when the server rate-limits the client.
type RateLimitError struct {
	StatusCode  int
	RetryAfter  time.Duration // zero if unknown
	URL         string
	Attempt     int
	MaxAttempts int
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("rssdetector: rate limited (HTTP %d) for %s after %d attempts; retry after %s",
			e.StatusCode, e.URL, e.Attempt, e.RetryAfter)
	}
	return fmt.Sprintf("rssdetector: rate limited (HTTP %d) for %s after %d attempts",
		e.StatusCode, e.URL, e.Attempt)
}

func (e *RateLimitError) Unwrap() error { return ErrRateLimited }

// BlockedError is returned when a CAPTCHA/challenge or soft-block is detected.
type BlockedError struct {
	StatusCode int
	URL        string
	Reason     string
}

func (e *BlockedError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("rssdetector: blocked (HTTP %d) for %s: %s", e.StatusCode, e.URL, e.Reason)
	}
	return fmt.Sprintf("rssdetector: blocked (HTTP %d) for %s", e.StatusCode, e.URL)
}

func (e *BlockedError) Unwrap() error { return ErrBlocked }

// FetchError wraps a non-recoverable HTTP/network failure.
type FetchError struct {
	StatusCode int
	URL        string
	Err        error
}

func (e *FetchError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("rssdetector: fetch failed for %s: HTTP %d", e.URL, e.StatusCode)
	}
	if e.Err != nil {
		return fmt.Sprintf("rssdetector: fetch failed for %s: %v", e.URL, e.Err)
	}
	return fmt.Sprintf("rssdetector: fetch failed for %s", e.URL)
}

func (e *FetchError) Unwrap() error {
	if e.Err != nil {
		return e.Err
	}
	return ErrFetchFailed
}

// IsHTTPStatus reports whether err is a FetchError with the given status.
func IsHTTPStatus(err error, status int) bool {
	var fe *FetchError
	if errors.As(err, &fe) {
		return fe.StatusCode == status
	}
	return false
}

// StatusFromError extracts an HTTP status code from known error types, or 0.
func StatusFromError(err error) int {
	var re *RateLimitError
	if errors.As(err, &re) {
		return re.StatusCode
	}
	var be *BlockedError
	if errors.As(err, &be) {
		return be.StatusCode
	}
	var fe *FetchError
	if errors.As(err, &fe) {
		return fe.StatusCode
	}
	return 0
}

