package rssdetector

import (
	"net/http"
	"time"
)

// RetryConfig controls retry behaviour for transient failures.
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// defaultRetry is used when Retry is zero-valued.
func defaultRetry() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    5 * time.Second,
	}
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient sets a custom HTTP client. The client's Timeout is not
// overridden; use WithTimeout for request-level deadlines via context.
func WithHTTPClient(c *http.Client) Option {
	return func(cl *Client) {
		if c != nil {
			cl.httpClient = c
		}
	}
}

// WithUserAgent sets the User-Agent header for outbound requests.
func WithUserAgent(ua string) Option {
	return func(cl *Client) {
		if ua != "" {
			cl.userAgent = ua
		}
	}
}

// WithTimeout sets the default per-Detect context timeout when the caller
// does not cancel earlier. Zero disables the package-level timeout.
func WithTimeout(d time.Duration) Option {
	return func(cl *Client) {
		cl.timeout = d
	}
}

// WithMaxRedirects limits HTTP redirects followed by the fetcher.
func WithMaxRedirects(n int) Option {
	return func(cl *Client) {
		if n >= 0 {
			cl.maxRedirects = n
		}
	}
}

// WithProbeCommonPaths enables/disables probing well-known feed paths when
// HTML discovery finds nothing.
func WithProbeCommonPaths(enabled bool) Option {
	return func(cl *Client) {
		cl.probeCommonPaths = enabled
	}
}

// WithConfirmFeedLinks enables lightweight content-type/prefix confirmation
// of candidate feed URLs (especially path probes). Never parses feed items.
func WithConfirmFeedLinks(enabled bool) Option {
	return func(cl *Client) {
		cl.confirmFeedLinks = enabled
	}
}

// WithMaxConcurrentProbes sets the concurrency limit for path probes.
func WithMaxConcurrentProbes(n int) Option {
	return func(cl *Client) {
		if n > 0 {
			cl.maxConcurrentProbes = n
		}
	}
}

// WithRetry configures retry/backoff for 429/503 and transient network errors.
func WithRetry(cfg RetryConfig) Option {
	return func(cl *Client) {
		if cfg.MaxAttempts > 0 {
			cl.retry.MaxAttempts = cfg.MaxAttempts
		}
		if cfg.BaseDelay > 0 {
			cl.retry.BaseDelay = cfg.BaseDelay
		}
		if cfg.MaxDelay > 0 {
			cl.retry.MaxDelay = cfg.MaxDelay
		}
	}
}

// WithPlatformHandlers enables built-in platform handlers (YouTube, etc.).
func WithPlatformHandlers(enabled bool) Option {
	return func(cl *Client) {
		cl.platformHandlers = enabled
	}
}

// WithMinRequestInterval sets a minimum delay between requests to the same host.
func WithMinRequestInterval(d time.Duration) Option {
	return func(cl *Client) {
		if d >= 0 {
			cl.minRequestInterval = d
		}
	}
}

// WithConfirmHTMLLinks, when true, also confirms HTML <link> discoveries with
// a second fetch (stricter; default false).
func WithConfirmHTMLLinks(enabled bool) Option {
	return func(cl *Client) {
		cl.confirmHTMLLinks = enabled
	}
}

// WithMaxBodyBytes caps how many bytes are read for classification/confirmation.
func WithMaxBodyBytes(n int64) Option {
	return func(cl *Client) {
		if n > 0 {
			cl.maxBodyBytes = n
		}
	}
}

func applyDefaults(cl *Client) {
	if cl.httpClient == nil {
		cl.httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	if cl.userAgent == "" {
		cl.userAgent = "rssdetector/1.0 (+https://github.com/rss_detector)"
	}
	if cl.timeout == 0 {
		cl.timeout = 30 * time.Second
	}
	if cl.maxRedirects == 0 {
		cl.maxRedirects = 10
	}
	// probeCommonPaths and confirmFeedLinks default true; set in New via zero-value tricks
	if cl.maxConcurrentProbes == 0 {
		cl.maxConcurrentProbes = 4
	}
	if cl.retry.MaxAttempts == 0 {
		cl.retry = defaultRetry()
	}
	if cl.maxBodyBytes == 0 {
		cl.maxBodyBytes = 32 * 1024
	}
}
