package rssdetector

import (
	"bytes"
	"net/http"
	"strings"
	"unicode"
)

// ContentKind is the high-level classification of a response body.
type ContentKind int

const (
	ContentUnknown ContentKind = iota
	ContentHTML
	ContentFeed
	ContentChallenge
	ContentOther
)

// Classification is the result of sniffing headers + body prefix.
type Classification struct {
	Kind     ContentKind
	FeedType FeedType
	Reason   string // for challenge detection
}

// ClassifyResponse sniffs Content-Type and body to determine content kind.
// body may be a prefix only (first N bytes).
func ClassifyResponse(contentType string, statusCode int, body []byte) Classification {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}

	// Challenge/block heuristics first for HTML-like responses.
	if isChallenge(statusCode, ct, body) {
		return Classification{Kind: ContentChallenge, Reason: challengeReason(body)}
	}

	// Feed by Content-Type.
	if ft, ok := feedTypeFromMIME(ct); ok {
		// Ensure body isn't clearly HTML error page.
		if looksLikeHTML(body) && !looksLikeFeed(body) {
			return Classification{Kind: ContentHTML}
		}
		return Classification{Kind: ContentFeed, FeedType: ft}
	}

	// Body sniff.
	if looksLikeFeed(body) {
		return Classification{Kind: ContentFeed, FeedType: feedTypeFromBody(body)}
	}
	if looksLikeHTML(body) || strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml") {
		return Classification{Kind: ContentHTML}
	}
	if len(body) == 0 {
		return Classification{Kind: ContentUnknown}
	}
	return Classification{Kind: ContentOther}
}

func feedTypeFromMIME(ct string) (FeedType, bool) {
	switch {
	case ct == "application/rss+xml", ct == "application/rdf+xml", strings.Contains(ct, "rss"):
		return FeedTypeRSS, true
	case ct == "application/atom+xml", strings.Contains(ct, "atom"):
		return FeedTypeAtom, true
	case ct == "application/feed+json":
		return FeedTypeJSON, true
	case ct == "application/xml", ct == "text/xml", ct == "text/xml-external-parsed-entity":
		return FeedTypeUnknown, true
	case ct == "application/json":
		// Could be JSON Feed; caller may refine via body.
		return FeedTypeJSON, true
	default:
		return "", false
	}
}

func looksLikeHTML(body []byte) bool {
	s := trimBOM(bytes.TrimLeftFunc(body, unicode.IsSpace))
	if len(s) == 0 {
		return false
	}
	lower := bytes.ToLower(s)
	prefixes := [][]byte{
		[]byte("<!doctype html"),
		[]byte("<html"),
		[]byte("<head"),
		[]byte("<body"),
	}
	for _, p := range prefixes {
		if bytes.HasPrefix(lower, p) {
			return true
		}
	}
	// HTML comment then doctype
	if bytes.HasPrefix(lower, []byte("<!--")) {
		if bytes.Contains(lower[:min(512, len(lower))], []byte("<html")) ||
			bytes.Contains(lower[:min(512, len(lower))], []byte("<!doctype html")) {
			return true
		}
	}
	return false
}

func looksLikeFeed(body []byte) bool {
	s := trimBOM(bytes.TrimLeftFunc(body, unicode.IsSpace))
	if len(s) == 0 {
		return false
	}
	lower := bytes.ToLower(s)

	// XML declaration or feed roots
	if bytes.HasPrefix(lower, []byte("<?xml")) {
		// Peek further for feed markers
		window := lower
		if len(window) > 2048 {
			window = window[:2048]
		}
		if bytes.Contains(window, []byte("<rss")) ||
			bytes.Contains(window, []byte("<feed")) ||
			bytes.Contains(window, []byte("<rdf:rdf")) ||
			bytes.Contains(window, []byte("<rdf:RDF")) {
			return true
		}
		// generic xml might still be a feed; require feed-ish tags
		return false
	}
	if bytes.HasPrefix(lower, []byte("<rss")) ||
		bytes.HasPrefix(lower, []byte("<feed")) ||
		bytes.HasPrefix(lower, []byte("<rdf:rdf")) {
		return true
	}
	// JSON Feed: { ... "version": "https://jsonfeed.org/..." }
	if bytes.HasPrefix(lower, []byte("{")) {
		window := lower
		if len(window) > 4096 {
			window = window[:4096]
		}
		if bytes.Contains(window, []byte("jsonfeed.org")) ||
			(bytes.Contains(window, []byte(`"version"`)) && bytes.Contains(window, []byte("items"))) {
			return true
		}
	}
	return false
}

func feedTypeFromBody(body []byte) FeedType {
	s := trimBOM(bytes.TrimLeftFunc(body, unicode.IsSpace))
	lower := bytes.ToLower(s)
	window := lower
	if len(window) > 2048 {
		window = window[:2048]
	}
	if bytes.HasPrefix(lower, []byte("{")) || bytes.Contains(window, []byte("jsonfeed.org")) {
		return FeedTypeJSON
	}
	if bytes.Contains(window, []byte("<feed")) && !bytes.Contains(window, []byte("<rss")) {
		return FeedTypeAtom
	}
	if bytes.Contains(window, []byte("<rss")) || bytes.Contains(window, []byte("<rdf:rdf")) {
		return FeedTypeRSS
	}
	return FeedTypeUnknown
}

func isChallenge(status int, ct string, body []byte) bool {
	// Soft blocks often return 200 with challenge HTML.
	if status == http.StatusTooManyRequests {
		// 429 is rate limit, not always captcha; still treat body check separately.
		return false
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		if challengeReason(body) != "" || looksLikeHTML(body) {
			return challengeReason(body) != ""
		}
	}
	reason := challengeReason(body)
	if reason == "" {
		return false
	}
	// Only flag as challenge if HTML-like or explicit challenge CT.
	if looksLikeHTML(body) || strings.Contains(ct, "text/html") || len(body) < 64*1024 {
		return true
	}
	return false
}

func challengeReason(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	lower := strings.ToLower(string(body))
	// Cap scan size
	if len(lower) > 64*1024 {
		lower = lower[:64*1024]
	}
	patterns := []struct {
		sub    string
		reason string
	}{
		{"cf-browser-verification", "cloudflare browser verification"},
		{"challenge-platform", "cloudflare challenge"},
		{"just a moment...", "cloudflare interstitial"},
		{"attention required", "cloudflare attention required"},
		{"hcaptcha", "hcaptcha"},
		{"recaptcha", "recaptcha"},
		{"g-recaptcha", "recaptcha"},
		{"captcha", "captcha"},
		{"bot detection", "bot detection"},
		{"access denied", "access denied"},
		{"cf-challenge", "cloudflare challenge"},
		{"_cf_chl", "cloudflare challenge"},
		{"enable javascript and cookies to continue", "javascript challenge"},
		{"google.com/sorry", "google sorry interstitial"},
		{"/sorry/index", "google sorry interstitial"},
		{"unusual traffic from your computer network", "google unusual traffic"},
	}
	for _, p := range patterns {
		if strings.Contains(lower, p.sub) {
			return p.reason
		}
	}
	return ""
}

func trimBOM(b []byte) []byte {
	return bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
