package rssdetector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProbeCommonPaths(t *testing.T) {
	rss, _ := os.ReadFile(filepath.Join("testdata", "sample.rss"))
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mux.HandleFunc("/feed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write(rss)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})

	c := New(
		WithHTTPClient(srv.Client()),
		WithRetry(RetryConfig{MaxAttempts: 1}),
		WithMaxConcurrentProbes(4),
		WithPlatformHandlers(false),
	)
	u, _ := url.Parse(srv.URL + "/")
	links, err := c.ProbeCommonPaths(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) == 0 {
		t.Fatal("expected at least one feed")
	}
	found := false
	for _, l := range links {
		if l.Source == SourceCommonPath {
			found = true
		}
	}
	if !found {
		t.Fatalf("links: %+v", links)
	}
}

func TestProbeSkipsHTMLAtPath(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<!DOCTYPE html><html><body>not a feed</body></html>"))
	})

	c := New(WithHTTPClient(srv.Client()), WithRetry(RetryConfig{MaxAttempts: 1}), WithPlatformHandlers(false))
	u, _ := url.Parse(srv.URL)
	links, err := c.ProbeCommonPaths(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 0 {
		t.Fatalf("expected no feeds, got %+v", links)
	}
}

func TestProbeRespectsConcurrency(t *testing.T) {
	// Just ensure it completes without hang
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond)
		w.WriteHeader(404)
	}))
	defer srv.Close()
	c := New(
		WithHTTPClient(srv.Client()),
		WithRetry(RetryConfig{MaxAttempts: 1}),
		WithMaxConcurrentProbes(2),
		WithPlatformHandlers(false),
	)
	u, _ := url.Parse(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := c.ProbeCommonPaths(ctx, u)
	if err != nil {
		t.Fatal(err)
	}
}
