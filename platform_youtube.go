package rssdetector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	path := u.Path

	// Already a feed URL — no network needed.
	if strings.Contains(path, "/feeds/videos.xml") {
		return []FeedLink{{
			URL:        CanonicalFeedURL(u.String()),
			Type:       FeedTypeAtom,
			Source:     PlatformSource("youtube"),
			Confidence: 0.99,
			Title:      "YouTube",
		}}, nil
	}

	// /channel/UCxxxx — construct feed without scraping.
	if id := extractPathSegment(path, "/channel/"); id != "" && isChannelID(id) {
		return youtubeChannelFeed(id), nil
	}

	// Playlist (explicit playlist path or list= on any YT URL when it looks like a playlist id).
	if list := u.Query().Get("list"); list != "" {
		if isPlaylistID(list) && (strings.Contains(path, "/playlist") || u.Query().Get("v") == "") {
			return youtubePlaylistFeed(list), nil
		}
		// watch?v=...&list=PL... → prefer channel of the video; playlist is secondary.
		// Callers usually want the channel feed for a video link.
	}

	// Video IDs: /watch?v=, youtu.be/ID, /shorts/ID, /embed/ID, /live/ID
	// Prefer Innertube player API (avoids Google /sorry rate limits on watch HTML).
	if vid := youtubeVideoID(u); vid != "" {
		id, err := youtubeChannelIDFromVideo(ctx, c, vid)
		if err != nil {
			return nil, err
		}
		if id != "" {
			return youtubeChannelFeed(id), nil
		}
		return nil, nil
	}

	// Handles / custom URLs: /@handle, /c/Name, /user/Name — need page or resolve.
	if strings.HasPrefix(path, "/@") || strings.HasPrefix(path, "/c/") || strings.HasPrefix(path, "/user/") {
		return h.discoverFromHTML(ctx, c, u)
	}

	// Unknown YouTube path — try HTML best-effort.
	return h.discoverFromHTML(ctx, c, u)
}

func (h youtubeHandler) discoverFromHTML(ctx context.Context, c *Client, u *url.URL) ([]FeedLink, error) {
	res, err := c.fetchURL(ctx, u.String())
	if err != nil {
		return nil, err
	}
	body := string(res.Body)
	if ru, err := url.Parse(res.URL); err == nil {
		if list := ru.Query().Get("list"); list != "" && isPlaylistID(list) && strings.Contains(ru.Path, "playlist") {
			return youtubePlaylistFeed(list), nil
		}
		// Redirected to a video URL
		if vid := youtubeVideoID(ru); vid != "" {
			if id, err := youtubeChannelIDFromVideo(ctx, c, vid); err == nil && id != "" {
				return youtubeChannelFeed(id), nil
			}
		}
	}
	if id := extractYouTubeChannelID(body); id != "" {
		return youtubeChannelFeed(id), nil
	}
	return nil, nil
}

func youtubeChannelFeed(channelID string) []FeedLink {
	return []FeedLink{{
		URL:        "https://www.youtube.com/feeds/videos.xml?channel_id=" + channelID,
		Type:       FeedTypeAtom,
		Source:     PlatformSource("youtube"),
		Confidence: 0.95,
		Title:      "YouTube Channel",
	}}
}

func youtubePlaylistFeed(playlistID string) []FeedLink {
	return []FeedLink{{
		URL:        "https://www.youtube.com/feeds/videos.xml?playlist_id=" + playlistID,
		Type:       FeedTypeAtom,
		Source:     PlatformSource("youtube"),
		Confidence: 0.95,
		Title:      "YouTube Playlist",
	}}
}

func isPlaylistID(id string) bool {
	if id == "" {
		return false
	}
	// Common YouTube playlist prefixes
	for _, p := range []string{"PL", "UU", "LL", "FL", "OL", "RD", "SS"} {
		if strings.HasPrefix(id, p) {
			return true
		}
	}
	return len(id) >= 10
}

// youtubeVideoID extracts a video id from watch/shorts/embed/live/youtu.be URLs.
func youtubeVideoID(u *url.URL) string {
	if u == nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	path := strings.Trim(u.Path, "/")
	parts := strings.Split(path, "/")

	if host == "youtu.be" && len(parts) >= 1 && parts[0] != "" {
		return sanitizeVideoID(parts[0])
	}

	if v := u.Query().Get("v"); v != "" {
		return sanitizeVideoID(v)
	}

	if len(parts) >= 2 {
		switch strings.ToLower(parts[0]) {
		case "shorts", "embed", "live", "v":
			return sanitizeVideoID(parts[1])
		}
	}
	return ""
}

func sanitizeVideoID(id string) string {
	id = strings.TrimSpace(id)
	if i := strings.IndexAny(id, "?&#"); i >= 0 {
		id = id[:i]
	}
	// YouTube video IDs are typically 11 chars [A-Za-z0-9_-]
	if len(id) < 6 || len(id) > 20 {
		return ""
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return ""
		}
	}
	return id
}

// youtubeChannelIDFromVideo resolves channel id via Innertube player (preferred)
// then oEmbed author_url fallback (may need a second request for @handles).
func youtubeChannelIDFromVideo(ctx context.Context, c *Client, videoID string) (string, error) {
	id, innErr := youtubeChannelIDInnertube(ctx, c, videoID)
	if innErr == nil && id != "" {
		return id, nil
	}

	authorURL, oemErr := youtubeOEmbedAuthorURL(ctx, c, videoID)
	if oemErr == nil && authorURL != "" {
		resolved, resErr := youtubeResolveAuthorURL(ctx, c, authorURL)
		if resErr == nil && resolved != "" {
			return resolved, nil
		}
		if resErr != nil && innErr == nil {
			return "", resErr
		}
	}

	if innErr != nil {
		return "", innErr
	}
	if oemErr != nil {
		return "", oemErr
	}
	return "", nil
}

// youtubeChannelIDInnertube uses the public YouTube Innertube player endpoint.
// This avoids scraping /watch HTML (often rate-limited to google.com/sorry).
func youtubeChannelIDInnertube(ctx context.Context, c *Client, videoID string) (string, error) {
	payload := map[string]any{
		"context": map[string]any{
			"client": map[string]any{
				"clientName":    "WEB",
				"clientVersion": "2.20240101.00.00",
				"hl":            "en",
			},
		},
		"videoId": videoID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://www.youtube.com/youtubei/v1/player?prettyPrint=false",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-YouTube-Client-Name", "1")
	req.Header.Set("X-YouTube-Client-Version", "2.20240101.00.00")

	if err := c.waitHostInterval(ctx, "www.youtube.com"); err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", &FetchError{URL: req.URL.String(), Err: err}
	}
	defer resp.Body.Close()

	limit := c.maxBodyBytes
	if limit < 64*1024 {
		limit = 64 * 1024 // player JSON needs a bit more headroom
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return "", &FetchError{URL: req.URL.String(), Err: err}
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return "", &RateLimitError{StatusCode: resp.StatusCode, URL: req.URL.String(), Attempt: 1, MaxAttempts: 1}
	}
	if resp.StatusCode >= 400 {
		return "", &FetchError{StatusCode: resp.StatusCode, URL: req.URL.String()}
	}

	// Prefer videoDetails.channelId (the video owner).
	var parsed struct {
		VideoDetails struct {
			ChannelID string `json:"channelId"`
		} `json:"videoDetails"`
	}
	if err := json.Unmarshal(raw, &parsed); err == nil && isChannelID(parsed.VideoDetails.ChannelID) {
		return parsed.VideoDetails.ChannelID, nil
	}
	// Regex fallback if JSON shape changes slightly
	if m := channelIDRe.FindSubmatch(raw); len(m) > 1 {
		return string(m[1]), nil
	}
	return "", nil
}

func youtubeOEmbedAuthorURL(ctx context.Context, c *Client, videoID string) (string, error) {
	watch := "https://www.youtube.com/watch?v=" + url.QueryEscape(videoID)
	oembed := "https://www.youtube.com/oembed?format=json&url=" + url.QueryEscape(watch)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, oembed, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	if err := c.waitHostInterval(ctx, "www.youtube.com"); err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", &FetchError{URL: oembed, Err: err}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if err != nil {
		return "", &FetchError{URL: oembed, Err: err}
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", &RateLimitError{StatusCode: 429, URL: oembed, Attempt: 1, MaxAttempts: 1}
	}
	if resp.StatusCode >= 400 {
		return "", &FetchError{StatusCode: resp.StatusCode, URL: oembed}
	}

	var meta struct {
		AuthorURL string `json:"author_url"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		// regex fallback
		if m := authorURLRe.FindSubmatch(raw); len(m) > 1 {
			return string(m[1]), nil
		}
		return "", fmt.Errorf("rssdetector: oembed parse: %w", err)
	}
	return meta.AuthorURL, nil
}

// youtubeResolveAuthorURL turns channel/handle author URLs into a channel id.
func youtubeResolveAuthorURL(ctx context.Context, c *Client, authorURL string) (string, error) {
	au, err := url.Parse(authorURL)
	if err != nil {
		return "", ErrInvalidURL
	}
	if id := extractPathSegment(au.Path, "/channel/"); id != "" && isChannelID(id) {
		return id, nil
	}
	// /@handle or /c/ or /user/ — fetch that page (often less blocked than /watch)
	if strings.HasPrefix(au.Path, "/@") || strings.HasPrefix(au.Path, "/c/") || strings.HasPrefix(au.Path, "/user/") {
		res, err := c.fetchURL(ctx, au.String())
		if err != nil {
			return "", err
		}
		return extractYouTubeChannelID(string(res.Body)), nil
	}
	return "", nil
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

var (
	channelIDRe  = regexp.MustCompile(`"channelId"\s*:\s*"(UC[\w-]{20,})"`)
	externalIDRe = regexp.MustCompile(`"externalId"\s*:\s*"(UC[\w-]{20,})"`)
	browseIDRe   = regexp.MustCompile(`"browseId"\s*:\s*"(UC[\w-]{20,})"`)
	metaChannelRe  = regexp.MustCompile(`(?i)<meta[^>]+itemprop=["']channelId["'][^>]+content=["'](UC[\w-]{20,})["']`)
	metaChannelRe2 = regexp.MustCompile(`(?i)<meta[^>]+content=["'](UC[\w-]{20,})["'][^>]+itemprop=["']channelId["']`)
	linkChannelRe  = regexp.MustCompile(`(?i)https?://(?:www\.)?youtube\.com/channel/(UC[\w-]{20,})`)
	// Prefer channelMetadataRenderer.externalId when scraping channel pages.
	channelMetaExternalRe = regexp.MustCompile(`"channelMetadataRenderer"\s*:\s*\{[^}]*?"externalId"\s*:\s*"(UC[\w-]{20,})"`)
	authorURLRe           = regexp.MustCompile(`"author_url"\s*:\s*"([^"]+)"`)
)

func extractYouTubeChannelID(body string) string {
	// Priority: channel page metadata → externalId → meta tags → first channelId
	for _, re := range []*regexp.Regexp{
		channelMetaExternalRe,
		externalIDRe,
		metaChannelRe,
		metaChannelRe2,
		linkChannelRe,
		browseIDRe,
		channelIDRe,
	} {
		if m := re.FindStringSubmatch(body); len(m) > 1 {
			return m[1]
		}
	}
	return ""
}

func isChannelID(id string) bool {
	return strings.HasPrefix(id, "UC") && len(id) >= 22
}
