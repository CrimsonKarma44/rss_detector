package rssdetector

// FeedType identifies the syndication format of a feed link.
type FeedType string

const (
	FeedTypeRSS     FeedType = "rss"
	FeedTypeAtom    FeedType = "atom"
	FeedTypeJSON    FeedType = "json"
	FeedTypeUnknown FeedType = "unknown"
)

// DiscoverSource describes how a feed link was found.
type DiscoverSource string

const (
	SourceHTMLLink    DiscoverSource = "html_link"
	SourceHTTPHeader  DiscoverSource = "http_header"
	SourceCommonPath  DiscoverSource = "common_path"
	SourceDirectFeed  DiscoverSource = "direct_feed"
	SourceAnchor      DiscoverSource = "anchor"
	SourcePlatform    DiscoverSource = "platform" // prefix; platforms use platform:<name>
)

// PlatformSource returns a DiscoverSource for a named platform handler.
func PlatformSource(name string) DiscoverSource {
	return DiscoverSource("platform:" + name)
}

// FeedLink is a discovered subscribe URL — not feed content or items.
type FeedLink struct {
	URL        string
	Title      string
	Type       FeedType
	Source     DiscoverSource
	Confidence float64 // 0–1; higher if content-sniff confirmed
}

// Result holds the outcome of a Detect call (feed links only).
type Result struct {
	InputURL string
	FinalURL string
	Feeds    []FeedLink
	Warnings []string
}
