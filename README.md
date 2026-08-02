# rssdetector

Go package that discovers **RSS / Atom / JSON Feed subscription links** for a site URL.

**Output is feed URLs only** — never individual feed items or entries.

```go
links, err := rssdetector.Detect(ctx, "https://example.com")
// links[i].URL is the feed to subscribe to
```

| | |
|---|---|
| **Module** | [`github.com/CrimsonKarma44/rss_detector`](https://github.com/CrimsonKarma44/rss_detector) |
| **Package name** | `rssdetector` |
| **Latest release** | `v0.1.2` |
| **Go** | 1.22+ (module uses current toolchain) |

---

## Install

```bash
go get github.com/CrimsonKarma44/rss_detector@v0.1.2
# or
go get github.com/CrimsonKarma44/rss_detector@latest
```

```go
import "github.com/CrimsonKarma44/rss_detector"

// Use the package name (not the module path suffix):
links, err := rssdetector.Detect(ctx, url)
```

> **Do not** run `go get rss_detector` — module paths must include a host (`github.com/...`).

---

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/CrimsonKarma44/rss_detector"
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

links, err := client.Detect(ctx, "https://www.youtube.com/watch?v=VIDEO_ID")
// → https://www.youtube.com/feeds/videos.xml?channel_id=UC...
```

Also available: `DetectOne`, `DetectResult`, and `Client.Detect*`.

---

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

---

## YouTube (non-standard feeds)

YouTube does not expose conventional `<link rel="alternate">` feeds. This package builds them:

| Input | How the feed is resolved |
|-------|---------------------------|
| `/channel/UC…` | Construct `feeds/videos.xml?channel_id=UC…` (no scrape) |
| `/watch?v=…`, `youtu.be/…`, `/shorts/…`, `/embed/…` | **Innertube player API** → owner `channelId` (avoids scraping `/watch`, which Google often rate-limits) |
| `/@handle`, `/c/…`, `/user/…` | Fetch channel page and extract `channelId` / `externalId` |
| `/playlist?list=PL…` | Construct `feeds/videos.xml?playlist_id=PL…` |
| Already `/feeds/videos.xml?…` | Returned as-is |

**Example**

```text
https://www.youtube.com/watch?v=zZ5-KVDIaPg
→ https://www.youtube.com/feeds/videos.xml?channel_id=UCUyeluBRhGPCW4rPe_UvBZQ
```

Channel/playlist IDs are never invented. If none can be resolved, no YouTube feed is returned for that URL.

Watch HTML scraping is intentionally avoided: Google frequently returns **HTTP 429** and redirects to `google.com/sorry`.

---

## Errors

| Error | Meaning |
|-------|---------|
| `ErrInvalidURL` | Bad or non-http(s) URL |
| `ErrFetchFailed` | Network / HTTP error |
| `ErrRateLimited` | HTTP 429 / Google “sorry” interstitial (retries exhausted) |
| `ErrBlocked` | CAPTCHA / challenge / soft bot-block |
| `ErrNoFeeds` | Page OK but no feed links found |

Use `errors.Is`. Typed wrappers: `*RateLimitError`, `*BlockedError`, `*FetchError`.

---

## Rate limits & bot protection

**In scope**

- Exponential backoff + jitter, honor `Retry-After`
- Concurrent probe limits, optional per-host interval
- Detect Cloudflare / captcha-like pages → `ErrBlocked`
- Detect Google `/sorry` interstitials → `ErrRateLimited` (fail fast)

**Out of scope**

- Solving CAPTCHAs
- Headless browsers
- Proxy rotation (inject your own `http.Client` via `WithHTTPClient`)

---

## Options reference

| Option | Default |
|--------|---------|
| `WithHTTPClient` | `http.Client` 30s timeout |
| `WithUserAgent` | `rssdetector/1.0 …` |
| `WithTimeout` | 30s overall Detect timeout |
| `WithMaxRedirects` | 10 |
| `WithProbeCommonPaths` | true (when HTML finds nothing) |
| `WithConfirmFeedLinks` | true for path probes / non-YouTube candidates |
| `WithConfirmHTMLLinks` | false |
| `WithMaxConcurrentProbes` | 4 |
| `WithPlatformHandlers` | true |
| `WithMinRequestInterval` | 0 |
| `WithMaxBodyBytes` | 32 KiB |
| `WithRetry` | 3 attempts, 500ms–5s |

YouTube platform results are returned without a second confirm fetch (constructed feed URLs are well-known and re-fetching YouTube is often rate-limited).

---

## Testing

```bash
go test ./...
```

Default tests use `httptest` and fixtures only (no live network).

Optional live smoke tests (hits the public internet):

```bash
go test -tags=live ./...
```

---

## Versioning

Tags use [semantic versioning](https://semver.org/): `vMAJOR.MINOR.PATCH`.

| Tag | Notes |
|-----|--------|
| `v0.1.0` | **Broken** — wrong `module` path (`rss_detector`). Do not use. |
| `v0.1.1` | Correct module path; pre–YouTube Innertube fix. |
| `v0.1.2` | **Recommended** — README + YouTube watch/shorts/oEmbed/Innertube path. |

```bash
go get github.com/CrimsonKarma44/rss_detector@v0.1.2
```

---

## Local development (optional)

```bash
# In a consumer module:
go mod edit -replace=github.com/CrimsonKarma44/rss_detector=/path/to/rss_detector
go mod tidy
```

Remove the `replace` when you want the published module again.

---

## License

Use as you wish in your project.
