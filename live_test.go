//go:build live

package rssdetector

import (
	"context"
	"testing"
	"time"
)

// Live smoke tests — run with: go test -tags=live ./...
// These hit the public internet and may flake under rate limits.

func TestLiveExampleCom(t *testing.T) {
	c := New(WithTimeout(20 * time.Second))
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	// example.com typically has no feed — ensure we don't panic
	_, err := c.Detect(ctx, "https://example.com")
	if err != nil && err != ErrNoFeeds {
		t.Logf("example.com: %v", err)
	}
}

func TestLiveYouTubeChannel(t *testing.T) {
	c := New(WithTimeout(30*time.Second), WithConfirmFeedLinks(false))
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	// Well-known channel path form (Rick Astley)
	links, err := c.Detect(ctx, "https://www.youtube.com/channel/UCuAXFkgsw1L7xaCfnd5JJOw")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) == 0 {
		t.Fatal("expected youtube feed link")
	}
	t.Logf("youtube feed: %s", links[0].URL)
}
