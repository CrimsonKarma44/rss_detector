package rssdetector

import (
	"bytes"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// FindFeedsInHTML extracts feed links from HTML document bytes.
// base is the final page URL used for resolving relative hrefs.
func FindFeedsInHTML(body []byte, base *url.URL) []FeedLink {
	if base == nil || len(body) == 0 {
		return nil
	}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		// tokenizer fallback: still try partial parse
		return findFeedsTokenizer(body, base)
	}

	// Resolve base href if present.
	pageBase := base
	if bh := findBaseHref(doc); bh != "" {
		if resolved, err := ResolveURL(base, bh); err == nil {
			if u, err := url.Parse(resolved); err == nil {
				pageBase = u
			}
		}
	}

	var links []FeedLink
	var anchors []FeedLink
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "link":
				if fl, ok := feedFromLinkAttrs(n.Attr, pageBase); ok {
					links = append(links, fl)
				}
			case "a":
				if fl, ok := feedFromAnchorAttrs(n.Attr, pageBase, textContent(n)); ok {
					anchors = append(anchors, fl)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	// Cap anchors
	const maxAnchors = 10
	if len(anchors) > maxAnchors {
		anchors = anchors[:maxAnchors]
	}
	return append(links, anchors...)
}

func findBaseHref(n *html.Node) string {
	var href string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if href != "" {
			return
		}
		if n.Type == html.ElementNode && strings.ToLower(n.Data) == "base" {
			for _, a := range n.Attr {
				if strings.EqualFold(a.Key, "href") {
					href = a.Val
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return href
}

func feedFromLinkAttrs(attrs []html.Attribute, base *url.URL) (FeedLink, bool) {
	var rel, typ, href, title string
	for _, a := range attrs {
		switch strings.ToLower(a.Key) {
		case "rel":
			rel = strings.ToLower(a.Val)
		case "type":
			typ = a.Val
		case "href":
			href = a.Val
		case "title":
			title = a.Val
		}
	}
	if href == "" {
		return FeedLink{}, false
	}
	if !relLooksLikeFeed(rel) {
		return FeedLink{}, false
	}
	ft, ok := feedTypeFromLink(typ, href)
	if !ok {
		return FeedLink{}, false
	}
	abs, err := ResolveURL(base, href)
	if err != nil {
		return FeedLink{}, false
	}
	conf := 0.9
	if ft == FeedTypeUnknown {
		conf = 0.6
	}
	return FeedLink{
		URL:        abs,
		Title:      title,
		Type:       ft,
		Source:     SourceHTMLLink,
		Confidence: conf,
	}, true
}

func feedFromAnchorAttrs(attrs []html.Attribute, base *url.URL, text string) (FeedLink, bool) {
	var typ, href, title string
	for _, a := range attrs {
		switch strings.ToLower(a.Key) {
		case "type":
			typ = a.Val
		case "href":
			href = a.Val
		case "title":
			title = a.Val
		}
	}
	if href == "" {
		return FeedLink{}, false
	}
	ft, byType := feedTypeFromLink(typ, href)
	// Need type OR feed-like href + feed-like text
	textBlob := strings.ToLower(text + " " + title)
	feedText := strings.Contains(textBlob, "rss") ||
		strings.Contains(textBlob, "atom") ||
		strings.Contains(textBlob, "feed") ||
		strings.Contains(textBlob, "subscribe")
	hrefFeed := hrefLooksLikeFeed(href)
	if !byType && !(hrefFeed && feedText) {
		// pure type match without mime still requires path-ish feed
		if ft == FeedTypeUnknown && !hrefFeed {
			return FeedLink{}, false
		}
		if !byType {
			return FeedLink{}, false
		}
	}
	// if only by type with empty type and hrefFeed+text
	if typ == "" && !hrefFeed {
		return FeedLink{}, false
	}
	if !byType {
		ft = FeedTypeUnknown
	}
	abs, err := ResolveURL(base, href)
	if err != nil {
		return FeedLink{}, false
	}
	if title == "" {
		title = strings.TrimSpace(text)
	}
	return FeedLink{
		URL:        abs,
		Title:      title,
		Type:       ft,
		Source:     SourceAnchor,
		Confidence: 0.4,
	}, true
}

func relLooksLikeFeed(rel string) bool {
	// rel may be space-separated tokens
	for _, tok := range strings.Fields(rel) {
		tok = strings.ToLower(strings.TrimSpace(tok))
		if tok == "alternate" || tok == "feed" {
			return true
		}
	}
	return false
}

func feedTypeFromLink(typ, href string) (FeedType, bool) {
	ct := strings.ToLower(strings.TrimSpace(typ))
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	switch ct {
	case "application/rss+xml", "application/rdf+xml", "text/rss", "text/rss+xml":
		return FeedTypeRSS, true
	case "application/atom+xml", "text/atom+xml":
		return FeedTypeAtom, true
	case "application/feed+json":
		return FeedTypeJSON, true
	case "application/json":
		if hrefLooksLikeFeed(href) {
			return FeedTypeJSON, true
		}
		return FeedTypeUnknown, false
	case "application/xml", "text/xml":
		if hrefLooksLikeFeed(href) {
			return FeedTypeUnknown, true
		}
		// still accept alternate+xml as weak
		return FeedTypeUnknown, true
	case "":
		if hrefLooksLikeFeed(href) {
			return FeedTypeUnknown, true
		}
		return FeedTypeUnknown, false
	default:
		if strings.Contains(ct, "rss") {
			return FeedTypeRSS, true
		}
		if strings.Contains(ct, "atom") {
			return FeedTypeAtom, true
		}
		return FeedTypeUnknown, false
	}
}

func hrefLooksLikeFeed(href string) bool {
	h := strings.ToLower(href)
	// strip query for path checks
	path := h
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	markers := []string{
		"rss", "atom", "feed", ".xml", "rdf", "jsonfeed",
	}
	for _, m := range markers {
		if strings.Contains(path, m) {
			return true
		}
	}
	// query feed=
	if strings.Contains(h, "feed=rss") || strings.Contains(h, "feed=atom") || strings.Contains(h, "feed=rdf") {
		return true
	}
	return false
}

func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// findFeedsTokenizer is a fallback for unparseable documents (rarely used).
func findFeedsTokenizer(body []byte, base *url.URL) []FeedLink {
	z := html.NewTokenizer(bytes.NewReader(body))
	var links []FeedLink
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		t := z.Token()
		if !strings.EqualFold(t.Data, "link") {
			continue
		}
		if fl, ok := feedFromLinkAttrs(t.Attr, base); ok {
			links = append(links, fl)
		}
	}
	return links
}
