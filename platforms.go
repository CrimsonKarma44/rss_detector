package rssdetector

import (
	"context"
	"net/url"
)

// PlatformHandler discovers feed links for sites that do not follow standard autodiscovery.
type PlatformHandler interface {
	// Name returns a short identifier (e.g. "youtube").
	Name() string
	// Match reports whether this handler applies to u.
	Match(u *url.URL) bool
	// Discover returns feed links for u. May fetch HTML via the client's fetcher.
	Discover(ctx context.Context, c *Client, u *url.URL) ([]FeedLink, error)
}

// defaultPlatformHandlers returns built-in handlers in priority order.
func defaultPlatformHandlers() []PlatformHandler {
	return []PlatformHandler{
		youtubeHandler{},
		mediumHandler{},
		githubHandler{},
		redditHandler{},
		substackHandler{},
		bloggerHandler{},
		tumblrHandler{},
		// WordPress is probe-heavy; handled via path list + generator hints, not a hard match.
	}
}

// RegisterPlatformHandler appends a custom handler (after built-ins if enabled).
func (c *Client) RegisterPlatformHandler(h PlatformHandler) {
	if h == nil {
		return
	}
	c.handlers = append(c.handlers, h)
}
