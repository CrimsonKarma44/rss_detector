package rssdetector

import (
	"context"
	"net/url"
)

// Detect discovers feed links for rawURL.
func (c *Client) Detect(ctx context.Context, rawURL string) ([]FeedLink, error) {
	res, err := c.DetectResult(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	return res.Feeds, nil
}

// DetectOne returns the highest-ranked feed link or ErrNoFeeds.
func (c *Client) DetectOne(ctx context.Context, rawURL string) (FeedLink, error) {
	links, err := c.Detect(ctx, rawURL)
	if err != nil {
		return FeedLink{}, err
	}
	if len(links) == 0 {
		return FeedLink{}, ErrNoFeeds
	}
	return links[0], nil
}

// DetectResult runs the full discovery pipeline and returns metadata.
func (c *Client) DetectResult(ctx context.Context, rawURL string) (*Result, error) {
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	u, err := NormalizeInput(rawURL)
	if err != nil {
		return nil, err
	}

	result := &Result{
		InputURL: u.String(),
		FinalURL: u.String(),
	}

	var links []FeedLink

	// Stage 1: platform handlers (before generic scrape when they match hard)
	if c.platformHandlers && len(c.handlers) > 0 {
		for _, h := range c.handlers {
			if !h.Match(u) {
				continue
			}
			found, herr := h.Discover(ctx, c, u)
			if herr != nil {
				// Platform hard-fail only if it's block/rate-limit
				if isHardBlock(herr) {
					return nil, herr
				}
				result.Warnings = append(result.Warnings, h.Name()+": "+herr.Error())
				continue
			}
			if len(found) > 0 {
				links = append(links, found...)
				// YouTube-style handlers fully resolve feed URLs from IDs; return early
				// without a second confirm fetch (watch pages / feeds are often rate-limited).
				if h.Name() == "youtube" {
					result.Feeds = dedupeAndRank(links)
					if len(result.Feeds) == 0 {
						return result, ErrNoFeeds
					}
					return result, nil
				}
			}
		}
	}

	// Stage 2: fetch primary resource
	res, err := c.fetchURL(ctx, u.String())
	if err != nil {
		// If platform already found feeds, return those despite fetch fail
		if len(links) > 0 {
			result.Feeds = dedupeAndRank(links)
			result.Warnings = append(result.Warnings, "primary fetch: "+err.Error())
			return result, nil
		}
		return nil, err
	}
	result.FinalURL = res.URL
	finalURL, _ := url.Parse(res.URL)
	if finalURL == nil {
		finalURL = u
	}

	class := ClassifyResponse(res.ContentType, res.StatusCode, res.Body)

	// Already a feed document
	if class.Kind == ContentFeed {
		ft := class.FeedType
		if ft == "" {
			ft = FeedTypeUnknown
		}
		links = append(links, FeedLink{
			URL:        res.URL,
			Type:       ft,
			Source:     SourceDirectFeed,
			Confidence: 1.0,
		})
		result.Feeds = dedupeAndRank(links)
		return result, nil
	}

	if class.Kind == ContentChallenge {
		if len(links) > 0 {
			result.Feeds = dedupeAndRank(links)
			result.Warnings = append(result.Warnings, "challenge page: "+class.Reason)
			return result, nil
		}
		return nil, &BlockedError{StatusCode: res.StatusCode, URL: res.URL, Reason: class.Reason}
	}

	// Stage 3: HTML + Link headers
	if class.Kind == ContentHTML || looksLikeHTML(res.Body) {
		htmlLinks := FindFeedsInHTML(res.Body, finalURL)
		links = append(links, htmlLinks...)

		if vals := res.Header.Values("Link"); len(vals) > 0 {
			links = append(links, FindFeedsInLinkHeaders(vals, finalURL)...)
		}
	}

	// Stage 4: common path probes if empty (or always if exhaustive — we only when empty)
	if c.probeCommonPaths && !hasHighConfidence(links) {
		probed, perr := c.ProbeCommonPaths(ctx, finalURL)
		if perr != nil {
			if isHardBlock(perr) && len(links) == 0 {
				return nil, perr
			}
			result.Warnings = append(result.Warnings, "path probe: "+perr.Error())
		} else {
			links = append(links, probed...)
		}
	}

	// Stage 5: confirm
	if len(links) > 0 {
		confirmed, warns := c.ConfirmFeedLinks(ctx, links, true)
		result.Warnings = append(result.Warnings, warns...)
		// If confirm dropped everything, fall back to unconfirmed high-signal HTML links
		if len(confirmed) == 0 {
			for _, fl := range links {
				if fl.Source == SourceHTMLLink || fl.Source == SourceHTTPHeader || fl.Source == SourceDirectFeed {
					confirmed = append(confirmed, fl)
				}
			}
		}
		links = confirmed
	}

	// Stage 6: dedupe rank
	result.Feeds = dedupeAndRank(links)
	if len(result.Feeds) == 0 {
		return result, ErrNoFeeds
	}
	return result, nil
}

func hasHighConfidence(links []FeedLink) bool {
	for _, fl := range links {
		if fl.Source == SourceHTMLLink || fl.Source == SourceHTTPHeader || fl.Source == SourceDirectFeed {
			return true
		}
		if fl.Confidence >= 0.8 {
			return true
		}
	}
	return false
}

