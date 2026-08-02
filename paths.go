package rssdetector

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
)

// Common feed path candidates relative to site origin.
var commonFeedPaths = []string{
	"/feed",
	"/feed/",
	"/rss",
	"/rss.xml",
	"/atom.xml",
	"/feed.xml",
	"/index.xml",
	"/feeds/posts/default",
	"/?feed=rss",
	"/?feed=rss2",
	"/?feed=atom",
	"/rss/index.xml",
	"/comments/feed/",
	"/feed/atom/",
	"/feed/rss/",
}

// ProbeCommonPaths GETs well-known feed paths and returns those that sniff as feeds.
func (c *Client) ProbeCommonPaths(ctx context.Context, base *url.URL) ([]FeedLink, error) {
	if base == nil {
		return nil, ErrInvalidURL
	}
	origin := Origin(base)
	if origin == "" {
		return nil, ErrInvalidURL
	}

	// Also try parent of deep paths for subdir blogs: e.g. /blog/post -> /blog/feed
	candidates := make([]string, 0, len(commonFeedPaths)+len(commonFeedPaths))
	seen := make(map[string]struct{})
	add := func(abs string) {
		if _, ok := seen[abs]; ok {
			return
		}
		seen[abs] = struct{}{}
		candidates = append(candidates, abs)
	}
	for _, p := range commonFeedPaths {
		add(origin + p)
	}
	// subdir: if path has multiple segments, try under first segment
	path := strings.Trim(base.Path, "/")
	if path != "" {
		segs := strings.Split(path, "/")
		if len(segs) >= 1 && segs[0] != "" {
			prefix := origin + "/" + segs[0]
			for _, p := range []string{"/feed", "/rss.xml", "/atom.xml", "/feed.xml"} {
				add(prefix + p)
			}
		}
	}

	type result struct {
		link FeedLink
		err  error
	}

	sem := make(chan struct{}, c.maxConcurrentProbes)
	if cap(sem) == 0 {
		sem = make(chan struct{}, 1)
	}
	var wg sync.WaitGroup
	ch := make(chan result, len(candidates))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var blockOnce sync.Once
	var blockErr error

	for _, cand := range candidates {
		cand := cand
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			if ctx.Err() != nil {
				return
			}
			res, err := c.fetchURL(ctx, cand)
			if err != nil {
				if isHardBlock(err) {
					blockOnce.Do(func() {
						blockErr = err
						cancel()
					})
				}
				return
			}
			class := ClassifyResponse(res.ContentType, res.StatusCode, res.Body)
			if class.Kind != ContentFeed {
				return
			}
			ft := class.FeedType
			if ft == "" {
				ft = FeedTypeUnknown
			}
			ch <- result{link: FeedLink{
				URL:        res.URL,
				Type:       ft,
				Source:     SourceCommonPath,
				Confidence: 0.75,
			}}
		}()
	}
	wg.Wait()
	close(ch)

	if blockErr != nil {
		return nil, blockErr
	}

	var links []FeedLink
	for r := range ch {
		if r.err != nil {
			continue
		}
		links = append(links, r.link)
	}
	return links, nil
}

func isHardBlock(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrBlocked) || errors.Is(err, ErrRateLimited)
}
