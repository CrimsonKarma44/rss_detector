package rssdetector

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func readTD(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestFindFeedsInHTML_RSS(t *testing.T) {
	base, _ := url.Parse("https://example.com/blog/")
	links := FindFeedsInHTML(readTD(t, "with_rss_link.html"), base)
	if len(links) != 1 {
		t.Fatalf("got %d links: %+v", len(links), links)
	}
	if links[0].URL != "https://example.com/feed.xml" {
		t.Fatalf("url = %s", links[0].URL)
	}
	if links[0].Type != FeedTypeRSS {
		t.Fatalf("type = %s", links[0].Type)
	}
	if links[0].Title != "RSS Feed" {
		t.Fatalf("title = %s", links[0].Title)
	}
	if links[0].Source != SourceHTMLLink {
		t.Fatalf("source = %s", links[0].Source)
	}
}

func TestFindFeedsInHTML_Atom(t *testing.T) {
	base, _ := url.Parse("https://example.com/")
	links := FindFeedsInHTML(readTD(t, "with_atom_link.html"), base)
	if len(links) != 1 {
		t.Fatalf("got %d", len(links))
	}
	if links[0].URL != "https://example.com/atom.xml" {
		t.Fatalf("url = %s", links[0].URL)
	}
	if links[0].Type != FeedTypeAtom {
		t.Fatalf("type = %s", links[0].Type)
	}
}

func TestFindFeedsInHTML_Relative(t *testing.T) {
	base, _ := url.Parse("https://example.com/blog/post")
	links := FindFeedsInHTML(readTD(t, "relative_feed.html"), base)
	if len(links) != 1 {
		t.Fatalf("got %d", len(links))
	}
	if links[0].URL != "https://example.com/blog/rss.xml" {
		t.Fatalf("url = %s", links[0].URL)
	}
}

func TestFindFeedsInHTML_BaseHref(t *testing.T) {
	base, _ := url.Parse("https://example.com/page")
	links := FindFeedsInHTML(readTD(t, "base_href.html"), base)
	if len(links) != 1 {
		t.Fatalf("got %d", len(links))
	}
	if links[0].URL != "https://cdn.example.com/blog/feed.xml" {
		t.Fatalf("url = %s", links[0].URL)
	}
}

func TestFindFeedsInHTML_Multi(t *testing.T) {
	base, _ := url.Parse("https://example.com/")
	links := FindFeedsInHTML(readTD(t, "multi_feeds.html"), base)
	// 3 link tags + 1 anchor
	if len(links) < 3 {
		t.Fatalf("got %d: %+v", len(links), links)
	}
	types := map[FeedType]bool{}
	for _, l := range links {
		types[l.Type] = true
	}
	if !types[FeedTypeRSS] || !types[FeedTypeAtom] || !types[FeedTypeJSON] {
		t.Fatalf("missing types: %v", types)
	}
}

func TestFindFeedsInHTML_None(t *testing.T) {
	base, _ := url.Parse("https://example.com/")
	links := FindFeedsInHTML(readTD(t, "no_feeds.html"), base)
	if len(links) != 0 {
		t.Fatalf("got %v", links)
	}
}

func TestFindFeedsInHTML_Malformed(t *testing.T) {
	base, _ := url.Parse("https://example.com/")
	// unclosed tags still parseable by x/net/html
	html := []byte(`<html><head><link rel="alternate" type="application/rss+xml" href="/f.xml"><body>`)
	links := FindFeedsInHTML(html, base)
	if len(links) != 1 {
		t.Fatalf("got %d", len(links))
	}
}
