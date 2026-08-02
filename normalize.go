package rssdetector

import (
	"net"
	"net/url"
	"strings"
)

// NormalizeInput prepares a user-supplied URL string for fetching.
// - Trims whitespace
// - Adds https:// if scheme is missing
// - Rejects non-http(s) schemes
// - Strips fragments
func NormalizeInput(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrInvalidURL
	}

	// Scheme-relative or bare host: assume https.
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, ErrInvalidURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, ErrInvalidURL
	}
	if u.Host == "" {
		return nil, ErrInvalidURL
	}

	u.Fragment = ""
	return u, nil
}

// ResolveURL resolves ref against base, returning an absolute URL string.
func ResolveURL(base *url.URL, ref string) (string, error) {
	if base == nil {
		return "", ErrInvalidURL
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", ErrInvalidURL
	}
	ru, err := url.Parse(ref)
	if err != nil {
		return "", ErrInvalidURL
	}
	abs := base.ResolveReference(ru)
	abs.Fragment = ""
	return abs.String(), nil
}

// CanonicalFeedURL normalizes a feed URL for deduplication:
// scheme and host lower-cased, default ports stripped, fragments removed.
func CanonicalFeedURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return strings.TrimSpace(raw)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		u.Host = net.JoinHostPort(host, port)
	} else {
		u.Host = host
	}
	u.Fragment = ""
	// Preserve path as-is except empty path becomes /
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}

// Origin returns scheme://host for path probing.
func Origin(u *url.URL) string {
	if u == nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// IsYouTubeHost reports whether host is a YouTube domain.
func IsYouTubeHost(host string) bool {
	h := strings.ToLower(host)
	h = strings.TrimPrefix(h, "www.")
	switch h {
	case "youtube.com", "m.youtube.com", "music.youtube.com", "youtu.be":
		return true
	default:
		return strings.HasSuffix(h, ".youtube.com")
	}
}
