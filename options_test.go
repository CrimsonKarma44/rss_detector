package rssdetector

import (
	"net/http"
	"testing"
	"time"
)

func TestNewDefaults(t *testing.T) {
	c := New()
	if c.userAgent == "" {
		t.Fatal("expected default user agent")
	}
	if !c.probeCommonPaths {
		t.Fatal("probeCommonPaths should default true")
	}
	if !c.confirmFeedLinks {
		t.Fatal("confirmFeedLinks should default true")
	}
	if !c.platformHandlers {
		t.Fatal("platformHandlers should default true")
	}
	if c.maxConcurrentProbes != 4 {
		t.Fatalf("maxConcurrentProbes = %d, want 4", c.maxConcurrentProbes)
	}
	if c.retry.MaxAttempts != 3 {
		t.Fatalf("retry.MaxAttempts = %d", c.retry.MaxAttempts)
	}
	if c.maxBodyBytes != 32*1024 {
		t.Fatalf("maxBodyBytes = %d", c.maxBodyBytes)
	}
	if c.httpClient == nil {
		t.Fatal("nil http client")
	}
	if len(c.handlers) == 0 {
		t.Fatal("expected built-in platform handlers")
	}
}

func TestOptionsOverrides(t *testing.T) {
	hc := &http.Client{Timeout: 5 * time.Second}
	c := New(
		WithHTTPClient(hc),
		WithUserAgent("TestBot/1.0"),
		WithTimeout(10*time.Second),
		WithMaxRedirects(3),
		WithProbeCommonPaths(false),
		WithConfirmFeedLinks(false),
		WithMaxConcurrentProbes(2),
		WithRetry(RetryConfig{MaxAttempts: 1, BaseDelay: time.Millisecond, MaxDelay: time.Second}),
		WithPlatformHandlers(false),
		WithMinRequestInterval(50*time.Millisecond),
		WithConfirmHTMLLinks(true),
		WithMaxBodyBytes(4096),
	)
	if c.httpClient.Timeout != 5*time.Second {
		t.Fatal("custom client not applied")
	}
	if c.userAgent != "TestBot/1.0" {
		t.Fatalf("ua = %q", c.userAgent)
	}
	if c.timeout != 10*time.Second {
		t.Fatalf("timeout = %v", c.timeout)
	}
	if c.maxRedirects != 3 {
		t.Fatalf("maxRedirects = %d", c.maxRedirects)
	}
	if c.probeCommonPaths {
		t.Fatal("probe should be false")
	}
	if c.confirmFeedLinks {
		t.Fatal("confirm should be false")
	}
	if c.maxConcurrentProbes != 2 {
		t.Fatalf("probes = %d", c.maxConcurrentProbes)
	}
	if c.retry.MaxAttempts != 1 {
		t.Fatalf("attempts = %d", c.retry.MaxAttempts)
	}
	if c.platformHandlers {
		t.Fatal("platforms should be false")
	}
	if len(c.handlers) != 0 {
		t.Fatal("no handlers when platforms disabled")
	}
	if c.minRequestInterval != 50*time.Millisecond {
		t.Fatalf("interval = %v", c.minRequestInterval)
	}
	if !c.confirmHTMLLinks {
		t.Fatal("confirmHTMLLinks should be true")
	}
	if c.maxBodyBytes != 4096 {
		t.Fatalf("maxBody = %d", c.maxBodyBytes)
	}
}
