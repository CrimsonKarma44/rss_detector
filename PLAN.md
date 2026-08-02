# Execution Plan: `rss_detector` — Go RSS/Atom Feed Link Discovery

## Goal

Build a pure Go library that, given a site URL, discovers the site’s **RSS and/or Atom feed link(s)** (URLs only) via scraping and heuristics, returns them through a simple API, and degrades gracefully under rate limits and bot-protection (CAPTCHA/challenges).

### Scope boundary (explicit)

| In scope | Out of scope |
|----------|--------------|
| Find and return feed **URLs** (subscribe links) | Fetching or parsing **individual feed items/entries** |
| Identify whether a link is RSS vs Atom (from `type`, path, or light sniff) | Building a full feed reader / item list API |
| Optional light check that a candidate URL *is* a feed document | Downloading full feed bodies for content use |
| YouTube / platform URL construction for the feed link | Video/post metadata extraction beyond locating the feed URL |

**Product promise:** input a page/site URL → output zero or more **feed link URLs**. Callers subscribe to those URLs themselves.

### Success criteria

- One primary call: `Detect(ctx, url)` → feed links (or empty + clear error).
- Handles common HTML autodiscovery, well-known paths, and platform-specific patterns (especially **YouTube**).
- Configurable HTTP client, timeouts, retries, concurrency.
- Clear signals when a site is blocked (rate limit / CAPTCHA / challenge).
- Unit + integration-style tests for **every** public and internal capability.
- Iterative build: implement → test → green → next slice.

---

## Package identity

| Item | Choice |
|------|--------|
| Module path | `rss_detector` (local; already in `go.mod`) |
| Package name | `rssdetector` (Go convention: no underscore in package names) |
| Go version | As in `go.mod` (1.22+ compatible style) |
| Dependencies | Prefer **stdlib only**; `golang.org/x/net/html` is acceptable for robust HTML parsing. No headless browsers. |

---

## Public API (easy to use)

### Minimal happy path

```go
links, err := rssdetector.Detect(ctx, "https://example.com")
// links[i].URL is the RSS/Atom feed link — not feed items
// links[i].Title, links[i].Type (RSS|Atom|JSONFeed|Unknown) describe the link
```

### With options

```go
client := rssdetector.New(
    rssdetector.WithHTTPClient(httpClient),
    rssdetector.WithUserAgent("MyApp/1.0 (+https://example.com)"),
    rssdetector.WithTimeout(15*time.Second),
    rssdetector.WithMaxRedirects(10),
    rssdetector.WithProbeCommonPaths(true),      // default true
    rssdetector.WithConfirmFeedLinks(true),      // prefix/content-type sniff only
    rssdetector.WithMaxConcurrentProbes(4),
    rssdetector.WithRetry(rssdetector.RetryConfig{
        MaxAttempts: 3,
        BaseDelay:   500 * time.Millisecond,
        MaxDelay:    5 * time.Second,
    }),
    rssdetector.WithPlatformHandlers(true),      // YouTube, Medium, etc.
)

links, err := client.Detect(ctx, "https://www.youtube.com/@somechannel")
// e.g. https://www.youtube.com/feeds/videos.xml?channel_id=UC...
```

### Result types

```go
type FeedType string // "rss", "atom", "json", "unknown"

// FeedLink is a discovered subscribe URL — not feed content or items.
type FeedLink struct {
    URL        string
    Title      string         // from <link title="..."> when available
    Type       FeedType
    Source     DiscoverSource // "html_link", "http_header", "common_path", "platform:youtube", ...
    Confidence float64        // 0–1; higher if content-sniff confirmed
}

type Result struct {
    InputURL string
    FinalURL string     // after redirects
    Feeds    []FeedLink // feed links only
    Warnings []string
}
```

### Errors (sentinel / typed)

| Error | When |
|-------|------|
| `ErrInvalidURL` | Bad / non-http(s) URL |
| `ErrFetchFailed` | Network / non-recoverable HTTP |
| `ErrRateLimited` | HTTP 429 / Retry-After exhausted |
| `ErrBlocked` | CAPTCHA / bot challenge / soft-block page |
| `ErrNoFeeds` | Page fetched successfully but no feeds found |
| `ErrNotHTML` | Content is not HTML and not already a feed |

`Detect` returns feeds even if some probes fail (best-effort); only hard-fails when the **primary** page cannot be evaluated and no platform handler can resolve feeds.

Convenience:

```go
// First feed or error
feed, err := rssdetector.DetectOne(ctx, url)
```

---

## Discovery strategy (ordered pipeline)

For each input URL, run a **pipeline**. Short-circuit when high-confidence feeds are found **unless** options request exhaustive discovery.

### Stage 0 — Normalize input

1. Trim whitespace; reject empty.
2. Parse URL; require `http` or `https`.
3. If scheme missing, assume `https://`.
4. Strip fragments (`#...`); preserve query when meaningful (YouTube).
5. Optionally normalize trailing slash for path probes only (not for original fetch).

### Stage 1 — Platform special handlers (before generic scrape)

Match host + path patterns. These sites often **omit** standard `<link rel="alternate">` or bury channel IDs.

#### 1a. YouTube (priority — non-conventional)

| Input pattern | Resolution |
|---------------|------------|
| `youtube.com/feeds/videos.xml?...` | Already a feed → return as-is after validation |
| `youtube.com/channel/UC...` | `https://www.youtube.com/feeds/videos.xml?channel_id=UC...` |
| `youtube.com/c/Name`, `/user/Name`, `/@handle` | Fetch HTML; extract `channelId` / `externalId` from meta, `ytInitialData`, or `"channelId":"UC..."` |
| `youtube.com/playlist?list=PL...` | `.../feeds/videos.xml?playlist_id=PL...` |
| `youtu.be/...` | Resolve redirect → video page; channel feed from page (best-effort) |
| `youtube.com/watch?v=...` | Extract channel id from page → channel feed |

**Do not** invent feeds without a real channel/playlist id. Prefer parsing over guessing.

#### 1b. Other built-in platform handlers

| Platform | Host hints | Feed construction |
|----------|------------|-------------------|
| WordPress | Common; also `generator` meta | Prefer HTML links; else probe `/feed`, `/feed/`, `/?feed=rss2`, `/comments/feed/` |
| Medium | `medium.com`, `*.medium.com` | `medium.com/feed/@user`, publication paths, subdomain + `/feed` |
| Blogger | `blogspot.com` | `/feeds/posts/default` (Atom) |
| Tumblr | `*.tumblr.com` | `/rss` |
| GitHub | `github.com` | Commits: `.../commits.atom`; releases: `.../releases.atom`; tags: `.../tags.atom` |
| Reddit | `reddit.com` | Append `.rss` to listing URLs |
| Substack | `*.substack.com` | `/feed` |

Handlers are modular (`PlatformHandler` interface) so users can register custom ones later:

```go
type PlatformHandler interface {
    Match(u *url.URL) bool
    Discover(ctx context.Context, u *url.URL, fetcher Fetcher) ([]FeedLink, error)
}
```

### Stage 2 — Fetch primary resource

1. GET with browser-like but honest User-Agent (configurable).
2. Follow redirects (limit); record `FinalURL`.
3. Classify response:
   - **Already a feed** (Content-Type or body sniff) → return that URL as the feed.
   - **HTML** → continue.
   - **Challenge / block** → `ErrBlocked`.
   - **429** → retry / `ErrRateLimited`.
   - Other 4xx/5xx → error with status.

#### Content sniffing (feed vs HTML)

- Content-Type: `application/rss+xml`, `application/atom+xml`, `application/xml`, `text/xml`, `application/feed+json`, careful with `application/json`.
- Body prefixes: `<?xml`, `<rss`, `<feed`, `<rdf:RDF`, JSON Feed shape.
- Reject if body is clearly HTML (`<!DOCTYPE html`, `<html`).

### Stage 3 — HTML autodiscovery

Parse with `golang.org/x/net/html` (resilient to messy HTML).

#### `<link>` tags (primary)

Accept when `rel` contains `alternate` **and** type/href looks like a feed:

| Type (case-insensitive, ignore params) | FeedType |
|----------------------------------------|----------|
| `application/rss+xml` | RSS |
| `application/atom+xml` | Atom |
| `application/rdf+xml` | RSS (RDF) |
| `application/feed+json` | JSONFeed |
| empty type + href path contains `rss`/`atom`/`feed` | Unknown → optional validate |

Also:

- `rel="feed"` (rare legacy).
- Absolute vs relative `href` → resolve against base URL (`<base href>` if present, else final page URL).
- Multiple links → return all; dedupe by normalized URL.
- Capture `title` attribute when present.

#### HTTP `Link` header (secondary)

Parse `Link: </feed>; rel="alternate"; type="application/rss+xml"` (RFC 8288-style simple parser).

#### In-page anchors (tertiary, optional, lower confidence)

`<a href="...">` where type is a feed MIME type, or href + text/title looks like RSS/Atom/feed/subscribe. Cap candidates (e.g. 10) to avoid noise.

### Stage 4 — Common path probing (if enabled and feeds still empty)

Probe relative to **origin** (and optionally final path parent for blog-in-subdir installs):

```
/feed
/feed/
/rss
/rss.xml
/atom.xml
/feed.xml
/index.xml
/feeds/posts/default
/?feed=rss
/?feed=rss2
/?feed=atom
/rss/index.xml
```

Rules:

- Concurrent with `MaxConcurrentProbes`.
- Stop early if validated feed found (configurable).
- Only accept if response sniffs as feed (not HTML error pages).
- Respect rate-limit / block errors; abort remaining probes if blocked.

### Stage 5 — Confirm candidates are feed *links* (optional; not item retrieval)

Purpose: drop false positives (e.g. `/feed` returning an HTML page). **Not** for reading posts.

1. GET with a **small body limit** (e.g. first 8–32 KiB only).
2. Confirm via Content-Type + prefix sniff.
3. Stop reading immediately after classification — **do not** parse `<item>` / `<entry>`.
4. Upgrade `Type` and `Confidence` on link metadata only.

HTML `<link rel="alternate" type="application/rss+xml">` can skip this second fetch by default (strong signal).

### Stage 6 — Dedupe, rank, return **links only**

1. Normalize feed URLs (scheme/host lower-case, strip default ports, remove fragments).
2. Dedupe.
3. Sort: confirmed > platform > html_link > header > path probe; then RSS/Atom over unknown.
4. Return `[]FeedLink` / `Result` or `ErrNoFeeds` — **never** feed items.

---

## Rate limiting & bot control

### Client-side politeness

| Mechanism | Behavior |
|-----------|----------|
| Timeouts | Per-request + overall `ctx` |
| Retries | Exponential backoff + jitter on 429, 503, network blips |
| `Retry-After` | Honor header when present |
| Concurrency | Semaphore for probes |
| User-Agent | Configurable string; sensible default identifying the library |
| Optional delay | `WithMinRequestInterval` between requests to same host |

### Detection of blocks / CAPTCHA

Heuristics only (no browser automation):

1. Status codes: `401`, `403`, `429`, `503` with challenge-ish bodies.
2. Body / title patterns: `captcha`, `cf-browser-verification`, `challenge-platform`, `Just a moment...` (Cloudflare), `hcaptcha`, `recaptcha`, etc.
3. Soft blocks: tiny HTML page with challenge scripts and no real content / no feed links.

On detection:

- Return `ErrBlocked` wrapping status + reason.
- Do **not** attempt to solve CAPTCHAs.
- Surface `Retry-After` if any via error fields.

### Explicitly out of scope

- Headless Chrome / Playwright.
- CAPTCHA solving services.
- Residential proxy rotation (caller may supply custom `http.Client` / transport).

---

## Project layout

```
rss_detector/
├── PLAN.md                 // this document
├── go.mod
├── README.md
├── detector.go             // Detect, DetectOne, Client orchestration
├── options.go              // functional options
├── types.go                // FeedLink, Result, FeedType
├── errors.go               // sentinel + typed errors
├── fetch.go                // HTTP fetch, redirects, retries
├── classify.go             // content sniff: feed vs HTML vs challenge
├── htmlfind.go             // parse <link>, <base>, anchors
├── headers.go              // Link header parser
├── paths.go                // common path list + probe
├── normalize.go            // URL normalize / resolve
├── confirm.go              // feed-link confirmation (sniff only)
├── platforms/
│   ├── platforms.go        // registry + PlatformHandler
│   ├── youtube.go
│   ├── medium.go
│   ├── wordpress.go
│   ├── blogger.go
│   ├── tumblr.go
│   ├── github.go
│   ├── reddit.go
│   └── substack.go
├── testdata/               // static HTML/XML fixtures
└── *_test.go
```

---

## Testing strategy

### Principles

1. **No live network in default unit tests** — use `httptest.Server` and fixtures.
2. Optional build-tagged **live smoke tests** (`//go:build live`) for real sites — opt-in only.
3. Table-driven tests everywhere.
4. After each phase: `go test ./...` until green before the next phase.

### Test matrix by module

| Module | Cases |
|--------|-------|
| `normalize` | missing scheme, fragments, relative resolve, base tag, trailing slash |
| `classify` | RSS/Atom/JSON/HTML/empty/binary sniff; CAPTCHA fixtures; 429 handling |
| `htmlfind` | rss+xml, atom+xml, rdf, feed+json; relative/absolute; missing type; multi; base href; malformed HTML; no feeds |
| `headers` | single/multi Link; quoted params; ignore unrelated rel |
| `paths` | hit `/feed`; 404 skip; HTML-at-path skip; concurrency cap |
| `confirm` | candidate is feed doc vs HTML error page; prefix sniff only |
| `fetch` | redirects; timeout via context; Retry-After; max attempts; custom client |
| `youtube` | `/channel/UC`; `/@handle` fixture; playlist; already-feed URL; missing id → no invent |
| other platforms | path rewrite unit tests with fake hosts |
| `Detect` integration | full pipeline against `httptest`; blocked site; already-feed URL; platform route |
| Errors | sentinel equality with `errors.Is` |
| Options | defaults applied; overrides honored |

---

## Iterative build order (execution phases)

Each phase ends with **tests written + green** before moving on.

| Phase | Deliverable | Tests |
|-------|-------------|-------|
| **P0** | Module scaffold, types, errors, options, `Client` stub | Compile; option defaults; error sentinels |
| **P1** | URL normalize + resolve | Unit tests |
| **P2** | Content classifier (feed/HTML/challenge) | Fixture-based |
| **P3** | Fetcher (GET, redirect, retry, 429) | `httptest` |
| **P4** | HTML `<link>` + base + Link header discovery | Fixtures |
| **P5** | Common path probes + link confirmation (sniff only) | `httptest` multi-route |
| **P6** | `Detect` / `DetectOne` orchestration + ranking/dedupe | Integration tables |
| **P7** | YouTube platform handler | Fixtures + path unit tests |
| **P8** | Other platform handlers | Unit tests each |
| **P9** | Bot/rate-limit polish + README examples | Error path tests |
| **P10** | Optional `live` smoke tests (skipped by default) | Build-tag |

### PR grouping (for incremental reviewable commits)

| PR | Title | Depends on | Maps to phases |
|----|-------|------------|----------------|
| **PR 1** | Package foundation (types, errors, options, normalize) | — | P0–P1 |
| **PR 2** | Content classifier and HTTP fetcher | PR 1 | P2–P3 |
| **PR 3** | HTML autodiscovery and Link header parsing | PR 1 | P4 |
| **PR 4** | Path probes, confirmation, Detect orchestration | PR 2, PR 3 | P5–P6 |
| **PR 5** | Platform handlers (YouTube + others) | PR 4 | P7–P8 |
| **PR 6** | README, package docs, optional live smoke tests | PR 5 | P9–P10 |

---

## Design decisions & trade-offs

| Decision | Rationale |
|----------|-----------|
| Stdlib-first + `x/net/html` | Portable, light dependency story |
| No headless browser | Keeps package light; CAPTCHA unsolvable by design |
| Platform handlers first | YouTube etc. never expose conventional autodiscovery reliably |
| Confirm by default for guessed paths | Reduces false positives from path probing |
| Best-effort multi-feed return | Sites often expose posts + comments feeds |
| Custom `http.Client` injection | Callers handle proxies, cookies, TLS |
| Feed links only | Matches product promise; no feed-reader scope creep |

### Out of scope (v1)

- Any retrieval or exposure of individual feed items/entries.
- Full feed body download for consumption; OPML export.
- Persistent cache / robots.txt enforcement (future option).
- Solving CAPTCHAs or bypassing Cloudflare JS challenges.

---

## Risks & mitigations

| Risk | Mitigation |
|------|------------|
| YouTube HTML structure changes | Multiple extraction strategies; fixture tests; optional live tag |
| Path probes look aggressive | Low concurrency, early stop, disable via option |
| False positive “feeds” | Validation + content sniff |
| Sites rate-limit probes | Retry-After, abort on block, host throttle |

---

## Defaults (unless you override)

1. **Module path**: `rss_detector` (already set).
2. **JSON Feed**: include as optional discoverable type (not only RSS/Atom).
3. **Default `ProbeCommonPaths`**: `true`, but only when HTML discovery finds nothing.
4. **Default `ConfirmFeedLinks`**: `true` for guessed paths; HTML autodiscovery links returned without extra fetch unless strict confirm enabled.
5. **Output**: feed **links only** — never items.
6. **CAPTCHA**: detect + return `ErrBlocked` only (no solver).
7. **Dependencies**: stdlib + `golang.org/x/net/html` only.

---

## Approval checklist

Please confirm or adjust before implementation:

- [x] Output is **feed URLs/links only** (not individual items)
- [x] API shape (`Detect` / `New` + options) looks right
- [x] YouTube + listed platforms are enough for v1 (or name more / fewer)
- [x] CAPTCHA policy: detect + error only (no solver) is acceptable
- [x] Stdlib + `golang.org/x/net/html` only is acceptable
- [x] JSON Feed in/out of scope
- [x] Module path preference if any (default: keep `rss_detector`)
- [x] Optional “confirm candidate is a feed document” (prefix sniff) is OK

---

## After approval

1. Implement phases P0→P10 iteratively on the current branch/stack.
2. Every phase: implement → write/extend tests → `go test ./...` until green.
3. No live network required for default CI tests.
4. README documents API, options, YouTube behavior, and bot-control limits.

**Reply with approval (and any checklist changes)** to start execution.
