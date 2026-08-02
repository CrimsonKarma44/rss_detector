package rssdetector

import (
	"net/url"
	"strings"
)

// FindFeedsInLinkHeaders parses RFC 8288-style Link headers for feed alternates.
func FindFeedsInLinkHeaders(headers []string, base *url.URL) []FeedLink {
	if base == nil {
		return nil
	}
	var out []FeedLink
	for _, h := range headers {
		out = append(out, parseLinkHeader(h, base)...)
	}
	return out
}

// parseLinkHeader parses a single Link header value (may contain multiple entries).
func parseLinkHeader(header string, base *url.URL) []FeedLink {
	var out []FeedLink
	// Split on commas that separate links — careful with quoted commas is rare in feed links.
	parts := splitLinkHeader(header)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// <url>; rel="alternate"; type="application/rss+xml"
		lt := strings.Index(part, "<")
		gt := strings.Index(part, ">")
		if lt < 0 || gt < 0 || gt <= lt {
			continue
		}
		ref := strings.TrimSpace(part[lt+1 : gt])
		params := part[gt+1:]
		rel, typ, title := "", "", ""
		for _, seg := range strings.Split(params, ";") {
			seg = strings.TrimSpace(seg)
			if seg == "" {
				continue
			}
			key, val, ok := strings.Cut(seg, "=")
			if !ok {
				continue
			}
			key = strings.ToLower(strings.TrimSpace(key))
			val = strings.TrimSpace(val)
			val = strings.Trim(val, `"`)
			switch key {
			case "rel":
				rel = strings.ToLower(val)
			case "type":
				typ = val
			case "title":
				title = val
			}
		}
		if !relLooksLikeFeed(rel) {
			continue
		}
		ft, ok := feedTypeFromLink(typ, ref)
		if !ok {
			continue
		}
		abs, err := ResolveURL(base, ref)
		if err != nil {
			continue
		}
		conf := 0.85
		if ft == FeedTypeUnknown {
			conf = 0.55
		}
		out = append(out, FeedLink{
			URL:        abs,
			Title:      title,
			Type:       ft,
			Source:     SourceHTTPHeader,
			Confidence: conf,
		})
	}
	return out
}

// splitLinkHeader splits a Link header on commas not inside <...>.
func splitLinkHeader(s string) []string {
	var parts []string
	var b strings.Builder
	inAngle := false
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '<':
			if !inQuote {
				inAngle = true
			}
			b.WriteByte(c)
		case '>':
			if !inQuote {
				inAngle = false
			}
			b.WriteByte(c)
		case '"':
			inQuote = !inQuote
			b.WriteByte(c)
		case ',':
			if !inAngle && !inQuote {
				parts = append(parts, b.String())
				b.Reset()
				continue
			}
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	if b.Len() > 0 {
		parts = append(parts, b.String())
	}
	return parts
}
