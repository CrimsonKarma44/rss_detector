package rssdetector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestConfirmFeedLinks(t *testing.T) {
	rss, _ := os.ReadFile(filepath.Join("testdata", "sample.rss"))
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mux.HandleFunc("/good.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write(rss)
	})
	mux.HandleFunc("/bad", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<!DOCTYPE html><html></html>"))
	})

	c := New(WithHTTPClient(srv.Client()), WithConfirmFeedLinks(true), WithRetry(RetryConfig{MaxAttempts: 1}), WithPlatformHandlers(false))
	in := []FeedLink{
		{URL: srv.URL + "/good.xml", Type: FeedTypeUnknown, Source: SourceCommonPath, Confidence: 0.5},
		{URL: srv.URL + "/bad", Type: FeedTypeUnknown, Source: SourceCommonPath, Confidence: 0.5},
		{URL: srv.URL + "/good.xml", Type: FeedTypeRSS, Source: SourceHTMLLink, Confidence: 0.9}, // skip confirm
	}
	out, warns := c.ConfirmFeedLinks(context.Background(), in, true)
	if len(out) < 2 {
		t.Fatalf("out=%+v warns=%v", out, warns)
	}
	// bad path probe dropped
	for _, fl := range out {
		if fl.URL == srv.URL+"/bad" {
			t.Fatal("bad feed should be dropped")
		}
	}
}

func TestConfirmSkipsHTMLLinkByDefault(t *testing.T) {
	// server should not be hit for HTML link
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	defer srv.Close()

	c := New(WithHTTPClient(srv.Client()), WithConfirmFeedLinks(true), WithConfirmHTMLLinks(false), WithPlatformHandlers(false))
	out, _ := c.ConfirmFeedLinks(context.Background(), []FeedLink{
		{URL: srv.URL + "/x", Source: SourceHTMLLink, Type: FeedTypeRSS, Confidence: 0.9},
	}, true)
	if hits != 0 {
		t.Fatalf("hits = %d", hits)
	}
	if len(out) != 1 {
		t.Fatalf("out = %+v", out)
	}
}
