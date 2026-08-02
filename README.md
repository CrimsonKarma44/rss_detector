# rssdetector

Go package that discovers **RSS / Atom / JSON Feed subscription links** for a site URL.

**Output is feed URLs only** — never individual feed items or entries.

```go
links, err := rssdetector.Detect(ctx, "https://example.com")
// links[i].URL is the feed to subscribe to
```

## Install

```bash
go get rss_detector
```

Module path is currently `rss_detector` (local). Package import name: `rssdetector`.

```go
import "rss_detector"
// package name: rssdetector
```

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"rss_detector"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	links, err := rssdetector.Detect(ctx, "https://example.com")
	if err != nil {
		log.Fatal(err)
	}
	for _, l := range links {
		fmt.Printf("%s  (%s, via %s)\n", l.URL, l.Type, l.Source)
	}
}
```

### Configurable client

```go
client := rssdetector.New(
	rssdetector.WithUserAgent("MyApp/1.0 (+https://example.com)"),
	rssdetector.WithTimeout(15*time.Second),
	rssdetector.WithProbeCommonPaths(true),
	rssdetector.WithConfirmFeedLinks(true),
	rssdetector.WithPlatformHandlers(true),
	rssdetector.WithMaxConcurrentProbes(4),
	rssdetector.WithRetry(rssdetector.RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    5 * time.Second,
	}),
)

links, err := client.Detect(ctx, "https://www.youtube.com/@somechannel")
// → https://www.youtube.com/feeds/videos.xml?channel_id=UC...
```

## How discovery works

1. **Normalize** the input URL (`https` default, strip fragments).
2. **Platform handlers** (YouTube, Medium, GitHub, Reddit, Substack, Blogger, Tumblr).
3. **Fetch** the page (redirects, retries, rate-limit / bot-block detection).
4. If the URL is already a feed document → return it.
5. **HTML autodiscovery** (`<link rel="alternate" type="application/rss+xml|atom+xml|…">`), `<base href>`, optional anchors.
6. **HTTP `Link` headers** (RFC 8288-style).
7. **Common path probes** (`/feed`, `/rss.xml`, `/atom.xml`, …) when HTML finds nothing.
8. **Light confirmation** (Content-Type + body prefix sniff — does **not** parse items).
9. **Dedupe + rank** feed links.

## YouTube (non-standard feeds)

YouTube does not expose conventional `<link rel="alternate">` feeds. This package builds them:

| Input | Feed link |
|-------|-----------|
| `/channel/UC…` | `https://www.youtube.com/feeds/videos.xml?channel_id=UC…` |
| `/@handle`, `/c/…`, `/user/…` | Scrape page for `channelId`, then same as above |
| `/playlist?list=PL…` | `…/feeds/videos.xml?playlist_id=PL…` |
| Already `/feeds/videos.xml?…` | Returned as-is |

Channel/playlist IDs are never invented; if none can be found, no feed is returned for that handler.

## Errors

| Error | Meaning |
|-------|---------|
| `ErrInvalidURL` | Bad or non-http(s) URL |
| `ErrFetchFailed` | Network / HTTP error |
| `ErrRateLimited` | HTTP 429 (retries exhausted) |
| `ErrBlocked` | CAPTCHA / challenge / soft bot-block |
| `ErrNoFeeds` | Page OK but no feed links found |

Use `errors.Is`. Typed wrappers: `*RateLimitError`, `*BlockedError`, `*FetchError`.

## Rate limits & bot protection

**In scope**

- Exponential backoff + jitter, honor `Retry-After`
- Concurrent probe limits, optional per-host interval
- Detect Cloudflare / captcha-like pages → `ErrBlocked`

**Out of scope**

- Solving CAPTCHAs
- Headless browsers
- Proxy rotation (inject your own `http.Client` via `WithHTTPClient`)

## Options reference

| Option | Default |
|--------|---------|
| `WithHTTPClient` | `http.Client` 30s timeout |
| `WithUserAgent` | `rssdetector/1.0 …` |
| `WithTimeout` | 30s overall Detect timeout |
| `WithMaxRedirects` | 10 |
| `WithProbeCommonPaths` | true (when HTML finds nothing) |
| `WithConfirmFeedLinks` | true for path/platform candidates |
| `WithConfirmHTMLLinks` | false |
| `WithMaxConcurrentProbes` | 4 |
| `WithPlatformHandlers` | true |
| `WithMinRequestInterval` | 0 |
| `WithMaxBodyBytes` | 32 KiB |
| `WithRetry` | 3 attempts, 500ms–5s |

## Testing

```bash
go test ./...
```

Default tests use `httptest` and fixtures only (no live network).

Optional live smoke tests:

```bash
go test -tags=live ./...
```

## License

Use as you wish in your project.
