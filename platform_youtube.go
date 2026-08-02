package rssdetector

import (
	"context"
	"net/url"
	"regexp"
	"strings"
)

type youtubeHandler struct{}

func (youtubeHandler) Name() string { return "youtube" }

func (youtubeHandler) Match(u *url.URL) bool {
	if u == nil {
		return false
	}
	return IsYouTubeHost(u.Host)
}

func (h youtubeHandler) Discover(ctx context.Context, c *Client, u *url.URL) ([]FeedLink, error) {
	if u == nil {
		return nil, ErrInvalidURL
	}

	// youtu.be short links — resolve via fetch then re-match path
	host := strings.ToLower(u.Hostname())
	if host == "youtu.be" {
		res, err := c.fetchURL(ctx, u.String())
		if err != nil {
			return nil, err
		}
		ru, err := url.Parse(res.URL)
		if err != nil {
			return nil, ErrInvalidURL
		}
		return h.Discover(ctx, c, ru)
	}

	path := u.Path
	// Already a feed URL
	if strings.Contains(path, "/feeds/videos.xml") {
		return []FeedLink{{
			URL:        CanonicalFeedURL(u.String()),
			Type:       FeedTypeAtom,
			Source:     PlatformSource("youtube"),
			Confidence: 0.99,
			Title:      "YouTube",
		}}, nil
	}

	// /channel/UCxxxx
	if id := extractPathSegment(path, "/channel/"); id != "" {
		if isChannelID(id) {
			return youtubeChannelFeed(id), nil
		}
	}

	// playlist
	if list := u.Query().Get("list"); list != "" && (strings.Contains(path, "/playlist") || list != "") {
		if strings.HasPrefix(list, "PL") || strings.HasPrefix(list, "UU") || strings.HasPrefix(list, "LL") ||
			strings.HasPrefix(list, "FL") || strings.HasPrefix(list, "OL") {
			return youtubePlaylistFeed(list), nil
		}
	}

	// Paths that need HTML scrape: /@handle, /c/Name, /user/Name, /watch
	needHTML := strings.HasPrefix(path, "/@") ||
		strings.HasPrefix(path, "/c/") ||
		strings.HasPrefix(path, "/user/") ||
		strings.HasPrefix(path, "/watch") ||
		path == "/watch"

	if !needHTML && !strings.Contains(path, "/channel/") {
		// Try HTML anyway for unknown youtube paths
		needHTML = true
	}

	if needHTML {
		res, err := c.fetchURL(ctx, u.String())
		if err != nil {
			return nil, err
		}
		body := string(res.Body)
		// playlist from final URL
		if ru, err := url.Parse(res.URL); err == nil {
			if list := ru.Query().Get("list"); list != "" && strings.Contains(ru.Path, "playlist") {
				return youtubePlaylistFeed(list), nil
			}
		}
		if id := extractYouTubeChannelID(body); id != "" {
			return youtubeChannelFeed(id), nil
		}
		// channel link in page
		if id := extractPathSegmentFromBody(body, "/channel/"); id != "" && isChannelID(id) {
			return youtubeChannelFeed(id), nil
		}
		return nil, nil
	}

	return nil, nil
}

func youtubeChannelFeed(channelID string) []FeedLink {
	feed := "https://www.youtube.com/feeds/videos.xml?channel_id=" + channelID
	return []FeedLink{{
		URL:        feed,
		Type:       FeedTypeAtom,
		Source:     PlatformSource("youtube"),
		Confidence: 0.95,
		Title:      "YouTube Channel",
	}}
}

func youtubePlaylistFeed(playlistID string) []FeedLink {
	feed := "https://www.youtube.com/feeds/videos.xml?playlist_id=" + playlistID
	return []FeedLink{{
		URL:        feed,
		Type:       FeedTypeAtom,
		Source:     PlatformSource("youtube"),
		Confidence: 0.95,
		Title:      "YouTube Playlist",
	}}
}

func extractPathSegment(path, prefix string) string {
	i := strings.Index(strings.ToLower(path), strings.ToLower(prefix))
	if i < 0 {
		return ""
	}
	rest := path[i+len(prefix):]
	if j := strings.IndexAny(rest, "/?#"); j >= 0 {
		rest = rest[:j]
	}
	return rest
}

var channelIDRe = regexp.MustCompile(`"channelId"\s*:\s*"(UC[\w-]{20,})"`)
var externalIDRe = regexp.MustCompile(`"externalId"\s*:\s*"(UC[\w-]{20,})"`)
var browseIDRe = regexp.MustCompile(`"browseId"\s*:\s*"(UC[\w-]{20,})"`)
var metaChannelRe = regexp.MustCompile(`(?i)<meta[^>]+itemprop=["']channelId["'][^>]+content=["'](UC[\w-]{20,})["']`)
var metaChannelRe2 = regexp.MustCompile(`(?i)<meta[^>]+content=["'](UC[\w-]{20,})["'][^>]+itemprop=["']channelId["']`)
var linkChannelRe = regexp.MustCompile(`(?i)https?://(?:www\.)?youtube\.com/channel/(UC[\w-]{20,})`)

func extractYouTubeChannelID(body string) string {
	for _, re := range []*regexp.Regexp{channelIDRe, externalIDRe, browseIDRe, metaChannelRe, metaChannelRe2, linkChannelRe} {
		if m := re.FindStringSubmatch(body); len(m) > 1 {
			return m[1]
		}
	}
	return ""
}

func extractPathSegmentFromBody(body, prefix string) string {
	// find first /channel/UC...
	return extractYouTubeChannelID(body)
}

func isChannelID(id string) bool {
	return strings.HasPrefix(id, "UC") && len(id) >= 22
}
