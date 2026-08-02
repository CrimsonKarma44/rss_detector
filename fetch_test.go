package rssdetector

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing UA")
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body>ok</body></html>"))
	}))
	defer srv.Close()

	c := New(WithHTTPClient(srv.Client()), WithTimeout(5*time.Second), WithRetry(RetryConfig{MaxAttempts: 1}))
	res, err := c.fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	if !looksLikeHTML(res.Body) {
		t.Fatal("expected html body")
	}
}

func TestFetchRedirect(t *testing.T) {
	mux := http.NewServeMux()
	final := ""
	srv := httptest.NewServer(mux)
	defer srv.Close()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+"/end", http.StatusFound)
	})
	mux.HandleFunc("/end", func(w http.ResponseWriter, r *http.Request) {
		final = "hit"
		w.Write([]byte("<html></html>"))
	})

	c := New(WithHTTPClient(srv.Client()), WithRetry(RetryConfig{MaxAttempts: 1}))
	res, err := c.fetch(context.Background(), srv.URL+"/start")
	if err != nil {
		t.Fatal(err)
	}
	if final != "hit" {
		t.Fatal("redirect not followed")
	}
	if res.URL != srv.URL+"/end" {
		t.Fatalf("final URL = %s", res.URL)
	}
}

func TestFetchRateLimit(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("slow down"))
	}))
	defer srv.Close()

	c := New(
		WithHTTPClient(srv.Client()),
		WithRetry(RetryConfig{MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond}),
	)
	_, err := c.fetch(context.Background(), srv.URL)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v", err)
	}
	if n.Load() < 2 {
		t.Fatalf("attempts = %d", n.Load())
	}
}

func TestFetchBlocked(t *testing.T) {
	body, _ := os.ReadFile(filepath.Join("testdata", "captcha_cloudflare.html"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(body)
	}))
	defer srv.Close()

	c := New(WithHTTPClient(srv.Client()), WithRetry(RetryConfig{MaxAttempts: 1}))
	_, err := c.fetch(context.Background(), srv.URL)
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("err = %v", err)
	}
}

func TestFetch404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("nope"))
	}))
	defer srv.Close()

	c := New(WithHTTPClient(srv.Client()), WithRetry(RetryConfig{MaxAttempts: 1}))
	_, err := c.fetch(context.Background(), srv.URL)
	if !errors.Is(err, ErrFetchFailed) {
		t.Fatalf("err = %v", err)
	}
	if !IsHTTPStatus(err, 404) {
		t.Fatalf("status: %v", err)
	}
}

func TestFetchContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte("late"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := New(WithHTTPClient(srv.Client()), WithRetry(RetryConfig{MaxAttempts: 1}), WithTimeout(0))
	// clear package timeout by setting very short - Client.timeout still applied in Detect not fetch
	c.timeout = 0
	_, err := c.fetch(ctx, srv.URL)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRetryAfter(t *testing.T) {
	if parseRetryAfter("3") != 3*time.Second {
		t.Fatal("seconds")
	}
	if parseRetryAfter("") != 0 {
		t.Fatal("empty")
	}
}
