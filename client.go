package rssdetector

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Client discovers RSS/Atom feed links for a site URL.
type Client struct {
	httpClient          *http.Client
	userAgent           string
	timeout             time.Duration
	maxRedirects        int
	probeCommonPaths    bool
	confirmFeedLinks    bool
	confirmHTMLLinks    bool
	platformHandlers    bool
	maxConcurrentProbes int
	minRequestInterval  time.Duration
	maxBodyBytes        int64
	retry               RetryConfig

	// host throttle
	mu       sync.Mutex
	lastReq  map[string]time.Time
	handlers []PlatformHandler
}

// New creates a Client with the given options.
func New(opts ...Option) *Client {
	cl := &Client{
		probeCommonPaths: true,
		confirmFeedLinks: true,
		platformHandlers: true,
		lastReq:          make(map[string]time.Time),
	}
	applyDefaults(cl)
	for _, opt := range opts {
		opt(cl)
	}
	// Re-apply non-zero defaults that options may have left empty incorrectly.
	if cl.httpClient == nil {
		cl.httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if cl.userAgent == "" {
		cl.userAgent = "rssdetector/1.0 (+https://github.com/rss_detector)"
	}
	if cl.maxRedirects <= 0 {
		cl.maxRedirects = 10
	}
	if cl.maxConcurrentProbes <= 0 {
		cl.maxConcurrentProbes = 4
	}
	if cl.retry.MaxAttempts <= 0 {
		cl.retry = defaultRetry()
	}
	if cl.maxBodyBytes <= 0 {
		cl.maxBodyBytes = 32 * 1024
	}
	if cl.platformHandlers {
		cl.handlers = defaultPlatformHandlers()
	}
	// Configure redirect limit on the transport-level client if using default CheckRedirect.
	cl.configureRedirects()
	return cl
}

func (c *Client) configureRedirects() {
	if c.httpClient == nil {
		return
	}
	// Copy client to avoid mutating a shared client the user passed in unexpectedly
	// when they already set CheckRedirect — only set if nil.
	max := c.maxRedirects
	if c.httpClient.CheckRedirect == nil {
		// Need a distinct client so we don't race-mutate caller's client.
		cloned := *c.httpClient
		cloned.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= max {
				return http.ErrUseLastResponse
			}
			return nil
		}
		c.httpClient = &cloned
	}
}

// waitHostInterval enforces minRequestInterval per host.
func (c *Client) waitHostInterval(ctx context.Context, host string) error {
	if c.minRequestInterval <= 0 {
		return nil
	}
	c.mu.Lock()
	last, ok := c.lastReq[host]
	var wait time.Duration
	if ok {
		elapsed := time.Since(last)
		if elapsed < c.minRequestInterval {
			wait = c.minRequestInterval - elapsed
		}
	}
	c.mu.Unlock()
	if wait > 0 {
		t := time.NewTimer(wait)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
	c.mu.Lock()
	c.lastReq[host] = time.Now()
	c.mu.Unlock()
	return nil
}

// package-level default client
var defaultClient = New()

// Detect discovers feed links for the given URL using the default client.
func Detect(ctx context.Context, rawURL string) ([]FeedLink, error) {
	return defaultClient.Detect(ctx, rawURL)
}

// DetectOne returns the first discovered feed link, or ErrNoFeeds.
func DetectOne(ctx context.Context, rawURL string) (FeedLink, error) {
	return defaultClient.DetectOne(ctx, rawURL)
}

// DetectResult runs full detection and returns a Result with metadata.
func DetectResult(ctx context.Context, rawURL string) (*Result, error) {
	return defaultClient.DetectResult(ctx, rawURL)
}
