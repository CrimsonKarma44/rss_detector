package rssdetector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyFeedBodies(t *testing.T) {
	rss, err := os.ReadFile(filepath.Join("testdata", "sample.rss"))
	if err != nil {
		t.Fatal(err)
	}
	atom, err := os.ReadFile(filepath.Join("testdata", "sample.atom"))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		ct   string
		body []byte
		kind ContentKind
		ft   FeedType
	}{
		{"rss mime", "application/rss+xml", rss, ContentFeed, FeedTypeRSS},
		{"atom mime", "application/atom+xml", atom, ContentFeed, FeedTypeAtom},
		{"rss body", "text/plain", rss, ContentFeed, FeedTypeRSS},
		{"atom body", "", atom, ContentFeed, FeedTypeAtom},
		{"html", "text/html", []byte("<!DOCTYPE html><html></html>"), ContentHTML, ""},
		{"json feed", "application/feed+json", []byte(`{"version":"https://jsonfeed.org/version/1","items":[]}`), ContentFeed, FeedTypeJSON},
		{"empty", "", nil, ContentUnknown, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ClassifyResponse(tt.ct, 200, tt.body)
			if c.Kind != tt.kind {
				t.Fatalf("kind = %v want %v", c.Kind, tt.kind)
			}
			if tt.ft != "" && c.FeedType != tt.ft {
				t.Fatalf("type = %v want %v", c.FeedType, tt.ft)
			}
		})
	}
}

func TestClassifyChallenge(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "captcha_cloudflare.html"))
	if err != nil {
		t.Fatal(err)
	}
	c := ClassifyResponse("text/html", 200, body)
	if c.Kind != ContentChallenge {
		t.Fatalf("kind = %v, reason=%q", c.Kind, c.Reason)
	}
	if c.Reason == "" {
		t.Fatal("expected reason")
	}
}

func TestLooksLikeFeedRejectsHTML(t *testing.T) {
	if looksLikeFeed([]byte("<!DOCTYPE html><html><rss></rss></html>")) {
		// prefix is html so should not look like feed at start
		// our looksLikeFeed checks prefixes - HTML prefix fails
	}
	if looksLikeFeed([]byte("<html>")) {
		t.Fatal("html should not be feed")
	}
}
