// Package rssdetector discovers RSS, Atom, and JSON Feed subscription links for a site URL.
//
// It returns feed link URLs only — never individual feed items or entries.
//
// Basic usage:
//
//	links, err := rssdetector.Detect(ctx, "https://example.com")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, l := range links {
//	    fmt.Println(l.URL, l.Type)
//	}
//
// The discovery pipeline normalizes the input URL, applies platform-specific
// handlers (YouTube, Medium, GitHub, Reddit, Substack, Blogger, Tumblr),
// fetches the page, parses HTML autodiscovery links and HTTP Link headers,
// optionally probes common feed paths, and lightly confirms candidates by
// content sniffing.
//
// Bot protection (CAPTCHA/challenges) and rate limits are detected and returned
// as ErrBlocked / ErrRateLimited; they are not solved or bypassed.
package rssdetector
