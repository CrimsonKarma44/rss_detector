package rssdetector

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDetectHTMLDiscovery(t *testing.T) {
	rss, _ := os.ReadFile(filepath.Join("testdata", "sample.rss"))
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/feed.xml" {
			w.Header().Set("Content-Type", "application/rss+xml")
			w.Write(rss)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><head>
			<link rel="alternate" type="application/rss+xml" title="Feed" href="/feed.xml">
		</head><body>hi</body></html>`))
	})

	c := New(
		WithHTTPClient(srv.Client()),
		WithPlatformHandlers(false),
		WithProbeCommonPaths(false),
		WithConfirmFeedLinks(false),
		WithRetry(RetryConfig{MaxAttempts: 1}),
		WithTimeout(5*time.Second),
	)
	links, err := c.Detect(context.Background(), srv.URL+"/")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("%+v", links)
	}
	if !strings.HasSuffix(links[0].URL, "/feed.xml") {
		t.Fatalf("%s", links[0].URL)
	}
}

func TestDetectPathOnly(t *testing.T) {
	rss, _ := os.ReadFile(filepath.Join("testdata", "sample.rss"))
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mux.HandleFunc("/feed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write(rss)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><head><title>x</title></head><body>no link tags</body></html>`))
	})

	c := New(
		WithHTTPClient(srv.Client()),
		WithPlatformHandlers(false),
		WithProbeCommonPaths(true),
		WithConfirmFeedLinks(true),
		WithRetry(RetryConfig{MaxAttempts: 1}),
		WithMaxConcurrentProbes(8),
	)
	links, err := c.Detect(context.Background(), srv.URL+"/")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) == 0 {
		t.Fatal("expected path probe hit")
	}
}

func TestDetectDirectFeed(t *testing.T) {
	rss, _ := os.ReadFile(filepath.Join("testdata", "sample.rss"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write(rss)
	}))
	defer srv.Close()

	c := New(WithHTTPClient(srv.Client()), WithPlatformHandlers(false), WithRetry(RetryConfig{MaxAttempts: 1}))
	links, err := c.Detect(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].Source != SourceDirectFeed {
		t.Fatalf("%+v", links)
	}
}

func TestDetectNoFeeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><body>nothing</body></html>`))
	}))
	defer srv.Close()

	c := New(
		WithHTTPClient(srv.Client()),
		WithPlatformHandlers(false),
		WithProbeCommonPaths(false),
		WithRetry(RetryConfig{MaxAttempts: 1}),
	)
	_, err := c.Detect(context.Background(), srv.URL)
	if !errors.Is(err, ErrNoFeeds) {
		t.Fatalf("err = %v", err)
	}
}

func TestDetectBlocked(t *testing.T) {
	body, _ := os.ReadFile(filepath.Join("testdata", "captcha_cloudflare.html"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(body)
	}))
	defer srv.Close()

	c := New(WithHTTPClient(srv.Client()), WithPlatformHandlers(false), WithRetry(RetryConfig{MaxAttempts: 1}))
	_, err := c.Detect(context.Background(), srv.URL)
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("err = %v", err)
	}
}

func TestDetectLinkHeader(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Link", `</feed.xml>; rel="alternate"; type="application/rss+xml"`)
		w.Write([]byte(`<!DOCTYPE html><html><body>x</body></html>`))
	})

	c := New(
		WithHTTPClient(srv.Client()),
		WithPlatformHandlers(false),
		WithProbeCommonPaths(false),
		WithConfirmFeedLinks(false),
		WithRetry(RetryConfig{MaxAttempts: 1}),
	)
	links, err := c.Detect(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].Source != SourceHTTPHeader {
		t.Fatalf("%+v", links)
	}
}

func TestDetectOne(t *testing.T) {
	rss, _ := os.ReadFile(filepath.Join("testdata", "sample.rss"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write(rss)
	}))
	defer srv.Close()
	c := New(WithHTTPClient(srv.Client()), WithPlatformHandlers(false), WithRetry(RetryConfig{MaxAttempts: 1}))
	fl, err := c.DetectOne(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if fl.URL == "" {
		t.Fatal("empty")
	}
}

func TestDetectInvalidURL(t *testing.T) {
	c := New(WithPlatformHandlers(false))
	_, err := c.Detect(context.Background(), "ftp://nope")
	if !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("%v", err)
	}
}

func TestDetectGitHubPlatform(t *testing.T) {
	// GitHub handler does not need network if we only construct URLs
	c := New(WithPlatformHandlers(true), WithProbeCommonPaths(false), WithConfirmFeedLinks(false), WithRetry(RetryConfig{MaxAttempts: 1}))
	// Discover via handler only — Detect will try to fetch github.com which may fail offline.
	// So call handler path through DetectResult after registering only github and skipping fetch...
	// Simpler: unit-level already covers github; here ensure Match + pipeline early path for medium-like without fetch success.
	h := githubHandler{}
	u, _ := urlParse("https://github.com/foo/bar")
	links, err := h.Discover(context.Background(), c, u)
	if err != nil || len(links) == 0 {
		t.Fatalf("%v %+v", err, links)
	}
}

func urlParse(s string) (*url.URL, error) {
	return url.Parse(s)
}
