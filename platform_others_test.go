package rssdetector

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

func TestMediumHandler(t *testing.T) {
	h := mediumHandler{}
	u, _ := url.Parse("https://medium.com/@alice/some-post")
	if !h.Match(u) {
		t.Fatal("match")
	}
	links, err := h.Discover(context.Background(), New(), u)
	if err != nil || len(links) != 1 {
		t.Fatalf("%v %+v", err, links)
	}
	if links[0].URL != "https://medium.com/feed/@alice" {
		t.Fatalf("%s", links[0].URL)
	}
	u2, _ := url.Parse("https://blog.medium.com/")
	links, _ = h.Discover(context.Background(), New(), u2)
	if len(links) != 1 || !strings.HasSuffix(links[0].URL, "/feed") {
		t.Fatalf("%+v", links)
	}
}

func TestGitHubHandler(t *testing.T) {
	h := githubHandler{}
	u, _ := url.Parse("https://github.com/golang/go")
	links, err := h.Discover(context.Background(), New(), u)
	if err != nil || len(links) < 2 {
		t.Fatalf("%v %+v", err, links)
	}
	u2, _ := url.Parse("https://github.com/golang/go/releases")
	links, _ = h.Discover(context.Background(), New(), u2)
	if len(links) != 1 || !strings.HasSuffix(links[0].URL, "/releases.atom") {
		t.Fatalf("%+v", links)
	}
}

func TestRedditHandler(t *testing.T) {
	h := redditHandler{}
	u, _ := url.Parse("https://www.reddit.com/r/golang/")
	links, err := h.Discover(context.Background(), New(), u)
	if err != nil || len(links) != 1 {
		t.Fatalf("%v %+v", err, links)
	}
	if !strings.HasSuffix(links[0].URL, "/r/golang.rss") {
		t.Fatalf("%s", links[0].URL)
	}
}

func TestSubstackHandler(t *testing.T) {
	h := substackHandler{}
	u, _ := url.Parse("https://example.substack.com/p/hello")
	links, err := h.Discover(context.Background(), New(), u)
	if err != nil || len(links) != 1 || links[0].URL != "https://example.substack.com/feed" {
		t.Fatalf("%v %+v", err, links)
	}
}

func TestBloggerHandler(t *testing.T) {
	h := bloggerHandler{}
	u, _ := url.Parse("https://myblog.blogspot.com/")
	links, err := h.Discover(context.Background(), New(), u)
	if err != nil || len(links) != 1 {
		t.Fatalf("%v %+v", err, links)
	}
	if !strings.HasSuffix(links[0].URL, "/feeds/posts/default") {
		t.Fatalf("%s", links[0].URL)
	}
}

func TestTumblrHandler(t *testing.T) {
	h := tumblrHandler{}
	u, _ := url.Parse("https://cool.tumblr.com/")
	links, err := h.Discover(context.Background(), New(), u)
	if err != nil || links[0].URL != "https://cool.tumblr.com/rss" {
		t.Fatalf("%v %+v", err, links)
	}
}
