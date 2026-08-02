package rssdetector

import (
	"context"
	"net/url"
	"strings"
)

// --- Medium ---

type mediumHandler struct{}

func (mediumHandler) Name() string { return "medium" }

func (mediumHandler) Match(u *url.URL) bool {
	if u == nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return h == "medium.com" || strings.HasSuffix(h, ".medium.com")
}

func (mediumHandler) Discover(ctx context.Context, c *Client, u *url.URL) ([]FeedLink, error) {
	h := strings.ToLower(u.Hostname())
	path := strings.Trim(u.Path, "/")
	parts := strings.Split(path, "/")

	var feed string
	switch {
	case h != "medium.com" && strings.HasSuffix(h, ".medium.com"):
		// publication subdomain
		feed = u.Scheme + "://" + u.Host + "/feed"
	case len(parts) >= 1 && strings.HasPrefix(parts[0], "@"):
		feed = "https://medium.com/feed/" + parts[0]
	case len(parts) >= 1 && parts[0] != "" && parts[0] != "feed":
		// publication path: medium.com/publication-name
		feed = "https://medium.com/feed/" + parts[0]
	default:
		return nil, nil
	}
	return []FeedLink{{
		URL:        feed,
		Type:       FeedTypeRSS,
		Source:     PlatformSource("medium"),
		Confidence: 0.85,
	}}, nil
}

// --- GitHub ---

type githubHandler struct{}

func (githubHandler) Name() string { return "github" }

func (githubHandler) Match(u *url.URL) bool {
	if u == nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return h == "github.com" || h == "www.github.com"
}

func (githubHandler) Discover(ctx context.Context, c *Client, u *url.URL) ([]FeedLink, error) {
	// Expect /owner/repo or deeper paths
	parts := splitPath(u.Path)
	if len(parts) < 2 {
		return nil, nil
	}
	owner, repo := parts[0], parts[1]
	if owner == "" || repo == "" || strings.HasPrefix(owner, ".") {
		return nil, nil
	}
	base := "https://github.com/" + owner + "/" + repo
	var links []FeedLink
	// Path-aware atom feeds
	switch {
	case len(parts) >= 3 && parts[2] == "commits":
		branch := "master"
		if len(parts) >= 4 {
			branch = parts[3]
		}
		links = append(links, FeedLink{
			URL: base + "/commits/" + branch + ".atom", Type: FeedTypeAtom,
			Source: PlatformSource("github"), Confidence: 0.9, Title: "Commits",
		})
	case len(parts) >= 3 && parts[2] == "releases":
		links = append(links, FeedLink{
			URL: base + "/releases.atom", Type: FeedTypeAtom,
			Source: PlatformSource("github"), Confidence: 0.9, Title: "Releases",
		})
	case len(parts) >= 3 && parts[2] == "tags":
		links = append(links, FeedLink{
			URL: base + "/tags.atom", Type: FeedTypeAtom,
			Source: PlatformSource("github"), Confidence: 0.9, Title: "Tags",
		})
	default:
		links = append(links,
			FeedLink{URL: base + "/commits.atom", Type: FeedTypeAtom, Source: PlatformSource("github"), Confidence: 0.8, Title: "Commits"},
			FeedLink{URL: base + "/releases.atom", Type: FeedTypeAtom, Source: PlatformSource("github"), Confidence: 0.8, Title: "Releases"},
			FeedLink{URL: base + "/tags.atom", Type: FeedTypeAtom, Source: PlatformSource("github"), Confidence: 0.75, Title: "Tags"},
		)
	}
	return links, nil
}

// --- Reddit ---

type redditHandler struct{}

func (redditHandler) Name() string { return "reddit" }

func (redditHandler) Match(u *url.URL) bool {
	if u == nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return h == "reddit.com" || h == "www.reddit.com" || h == "old.reddit.com" || strings.HasSuffix(h, ".reddit.com")
}

func (redditHandler) Discover(ctx context.Context, c *Client, u *url.URL) ([]FeedLink, error) {
	// Append .rss to listing URLs
	p := u.Path
	if strings.HasSuffix(p, ".rss") || strings.HasSuffix(p, ".json") {
		return []FeedLink{{
			URL: CanonicalFeedURL(u.String()), Type: FeedTypeRSS,
			Source: PlatformSource("reddit"), Confidence: 0.95,
		}}, nil
	}
	// strip trailing slash
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		p = "/.rss"
	} else {
		p = p + ".rss"
	}
	feed := u.Scheme + "://" + u.Host + p
	if u.RawQuery != "" {
		feed += "?" + u.RawQuery
	}
	return []FeedLink{{
		URL: feed, Type: FeedTypeRSS,
		Source: PlatformSource("reddit"), Confidence: 0.9,
	}}, nil
}

// --- Substack ---

type substackHandler struct{}

func (substackHandler) Name() string { return "substack" }

func (substackHandler) Match(u *url.URL) bool {
	if u == nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return strings.HasSuffix(h, ".substack.com") || h == "substack.com"
}

func (substackHandler) Discover(ctx context.Context, c *Client, u *url.URL) ([]FeedLink, error) {
	h := strings.ToLower(u.Hostname())
	if h == "substack.com" {
		// substack.com/@user or /profile — limited
		return nil, nil
	}
	feed := u.Scheme + "://" + u.Host + "/feed"
	return []FeedLink{{
		URL: feed, Type: FeedTypeRSS,
		Source: PlatformSource("substack"), Confidence: 0.9,
	}}, nil
}

// --- Blogger ---

type bloggerHandler struct{}

func (bloggerHandler) Name() string { return "blogger" }

func (bloggerHandler) Match(u *url.URL) bool {
	if u == nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return strings.Contains(h, "blogspot.") || h == "www.blogger.com" || h == "blogger.com"
}

func (bloggerHandler) Discover(ctx context.Context, c *Client, u *url.URL) ([]FeedLink, error) {
	if strings.Contains(strings.ToLower(u.Hostname()), "blogspot.") {
		feed := u.Scheme + "://" + u.Host + "/feeds/posts/default"
		return []FeedLink{{
			URL: feed, Type: FeedTypeAtom,
			Source: PlatformSource("blogger"), Confidence: 0.9,
		}}, nil
	}
	return nil, nil
}

// --- Tumblr ---

type tumblrHandler struct{}

func (tumblrHandler) Name() string { return "tumblr" }

func (tumblrHandler) Match(u *url.URL) bool {
	if u == nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return strings.HasSuffix(h, ".tumblr.com")
}

func (tumblrHandler) Discover(ctx context.Context, c *Client, u *url.URL) ([]FeedLink, error) {
	feed := u.Scheme + "://" + u.Host + "/rss"
	return []FeedLink{{
		URL: feed, Type: FeedTypeRSS,
		Source: PlatformSource("tumblr"), Confidence: 0.85,
	}}, nil
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}
