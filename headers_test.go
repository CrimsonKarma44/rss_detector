package rssdetector

import (
	"net/url"
	"testing"
)

func TestFindFeedsInLinkHeaders(t *testing.T) {
	base, _ := url.Parse("https://example.com/page")
	headers := []string{
		`</feed>; rel="alternate"; type="application/rss+xml"; title="RSS"`,
		`</atom.xml>; rel=alternate; type="application/atom+xml", </style.css>; rel="stylesheet"`,
	}
	links := FindFeedsInLinkHeaders(headers, base)
	if len(links) != 2 {
		t.Fatalf("got %d: %+v", len(links), links)
	}
	if links[0].URL != "https://example.com/feed" {
		t.Fatalf("url0 = %s", links[0].URL)
	}
	if links[0].Type != FeedTypeRSS {
		t.Fatalf("type0 = %s", links[0].Type)
	}
	if links[0].Title != "RSS" {
		t.Fatalf("title = %s", links[0].Title)
	}
	if links[1].URL != "https://example.com/atom.xml" {
		t.Fatalf("url1 = %s", links[1].URL)
	}
}

func TestLinkHeaderIgnoresUnrelated(t *testing.T) {
	base, _ := url.Parse("https://example.com/")
	links := FindFeedsInLinkHeaders([]string{`</next>; rel="next"`}, base)
	if len(links) != 0 {
		t.Fatalf("got %v", links)
	}
}

func TestSplitLinkHeaderQuoted(t *testing.T) {
	parts := splitLinkHeader(`</a>; title="x,y", </b>; rel="alternate"; type="application/rss+xml"`)
	if len(parts) != 2 {
		t.Fatalf("parts = %#v", parts)
	}
}
