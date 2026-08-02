package rssdetector

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// FetchResult is a single HTTP GET outcome used for discovery.
type FetchResult struct {
	URL         string // final URL after redirects
	StatusCode  int
	ContentType string
	Header      http.Header
	Body        []byte
}

// fetch performs a GET with retries, body limit, and block/rate-limit detection.
func (c *Client) fetch(ctx context.Context, rawURL string) (*FetchResult, error) {
	u, err := NormalizeInput(rawURL)
	if err != nil {
		return nil, err
	}
	return c.fetchURL(ctx, u.String())
}

func (c *Client) fetchURL(ctx context.Context, rawURL string) (*FetchResult, error) {
	maxAttempts := c.retry.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// Host throttle
		if host := hostOf(rawURL); host != "" {
			if err := c.waitHostInterval(ctx, host); err != nil {
				return nil, err
			}
		}

		res, err := c.doOnce(ctx, rawURL)
		if err != nil {
			lastErr = &FetchError{URL: rawURL, Err: err}
			if attempt < maxAttempts && isRetriableNet(err) {
				if err := c.sleepBackoff(ctx, attempt, 0); err != nil {
					return nil, err
				}
				continue
			}
			return nil, lastErr
		}

		// Classify blocks / rate limits from status + body.
		class := ClassifyResponse(res.ContentType, res.StatusCode, res.Body)

		if res.StatusCode == http.StatusTooManyRequests {
			ra := parseRetryAfter(res.Header.Get("Retry-After"))
			lastErr = &RateLimitError{
				StatusCode:  res.StatusCode,
				RetryAfter:  ra,
				URL:         res.URL,
				Attempt:     attempt,
				MaxAttempts: maxAttempts,
			}
			if attempt < maxAttempts {
				if err := c.sleepBackoff(ctx, attempt, ra); err != nil {
					return nil, err
				}
				continue
			}
			return nil, lastErr
		}

		if class.Kind == ContentChallenge {
			return nil, &BlockedError{
				StatusCode: res.StatusCode,
				URL:        res.URL,
				Reason:     class.Reason,
			}
		}

		// Retry 503
		if res.StatusCode == http.StatusServiceUnavailable && attempt < maxAttempts {
			ra := parseRetryAfter(res.Header.Get("Retry-After"))
			lastErr = &FetchError{StatusCode: res.StatusCode, URL: res.URL}
			if err := c.sleepBackoff(ctx, attempt, ra); err != nil {
				return nil, err
			}
			continue
		}

		if res.StatusCode >= 400 {
			// 403 without challenge patterns is still blocked-ish.
			if res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusUnauthorized {
				if reason := challengeReason(res.Body); reason != "" {
					return nil, &BlockedError{StatusCode: res.StatusCode, URL: res.URL, Reason: reason}
				}
			}
			return nil, &FetchError{StatusCode: res.StatusCode, URL: res.URL}
		}

		return res, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, &FetchError{URL: rawURL, Err: fmt.Errorf("exhausted retries")}
}

func (c *Client) doOnce(ctx context.Context, rawURL string) (*FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html, application/xhtml+xml, application/xml;q=0.9, application/rss+xml;q=0.8, application/atom+xml;q=0.8, application/feed+json;q=0.7, */*;q=0.5")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	limit := c.maxBodyBytes
	if limit <= 0 {
		limit = 32 * 1024
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		body = body[:limit]
	}

	finalURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}

	return &FetchResult{
		URL:         finalURL,
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		Header:      resp.Header.Clone(),
		Body:        body,
	}, nil
}

func (c *Client) sleepBackoff(ctx context.Context, attempt int, retryAfter time.Duration) error {
	delay := retryAfter
	if delay <= 0 {
		base := c.retry.BaseDelay
		if base <= 0 {
			base = 500 * time.Millisecond
		}
		max := c.retry.MaxDelay
		if max <= 0 {
			max = 5 * time.Second
		}
		// exponential: base * 2^(attempt-1) with jitter
		d := base * time.Duration(1<<uint(attempt-1))
		if d > max {
			d = max
		}
		// jitter ±25%
		j := time.Duration(rand.Int63n(int64(d/2)+1)) - d/4
		delay = d + j
		if delay < 0 {
			delay = d
		}
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	// seconds
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	// HTTP date
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
}

func hostOf(raw string) string {
	// quick parse
	if i := strings.Index(raw, "://"); i >= 0 {
		rest := raw[i+3:]
		if j := strings.IndexAny(rest, "/?#"); j >= 0 {
			return strings.ToLower(rest[:j])
		}
		return strings.ToLower(rest)
	}
	return ""
}

func isRetriableNet(err error) bool {
	if err == nil {
		return false
	}
	// context cancel is not retriable
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}
	s := err.Error()
	// common transient patterns
	for _, p := range []string{"timeout", "temporary", "connection reset", "EOF", "broken pipe", "tls handshake"} {
		if strings.Contains(strings.ToLower(s), p) {
			return true
		}
	}
	return false
}
