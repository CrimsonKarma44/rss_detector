package rssdetector

import "testing"

func TestDedupeAndRank(t *testing.T) {
	in := []FeedLink{
		{URL: "https://Example.com/feed", Source: SourceCommonPath, Confidence: 0.5, Type: FeedTypeRSS},
		{URL: "https://example.com/feed", Source: SourceHTMLLink, Confidence: 0.9, Type: FeedTypeRSS, Title: "Main"},
		{URL: "https://example.com/atom.xml", Source: SourceHTMLLink, Confidence: 0.9, Type: FeedTypeAtom},
	}
	out := dedupeAndRank(in)
	if len(out) != 2 {
		t.Fatalf("len = %d %+v", len(out), out)
	}
	// HTML link should win for /feed
	var feed FeedLink
	for _, fl := range out {
		if fl.URL == "https://example.com/feed" || fl.URL == CanonicalFeedURL("https://example.com/feed") {
			feed = fl
		}
	}
	if feed.Source != SourceHTMLLink {
		t.Fatalf("expected html_link winner: %+v", out)
	}
	if feed.Title != "Main" {
		t.Fatalf("title lost: %+v", feed)
	}
}
