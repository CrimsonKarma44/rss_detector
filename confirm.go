package rssdetector

import (
	"context"
)

// ConfirmFeedLinks optionally fetches each candidate and upgrades type/confidence
// when the body sniffs as a feed document. Never parses items/entries.
// Candidates that fail confirmation are dropped when dropInvalid is true.
func (c *Client) ConfirmFeedLinks(ctx context.Context, links []FeedLink, dropInvalid bool) ([]FeedLink, []string) {
	if !c.confirmFeedLinks && !c.confirmHTMLLinks {
		return links, nil
	}
	var out []FeedLink
	var warnings []string
	for _, fl := range links {
		// Skip confirmation for strong HTML autodiscovery unless strict mode.
		if fl.Source == SourceHTMLLink && !c.confirmHTMLLinks {
			out = append(out, fl)
			continue
		}
		if fl.Source == SourceHTTPHeader && !c.confirmHTMLLinks {
			out = append(out, fl)
			continue
		}
		if fl.Source == SourceDirectFeed {
			// already confirmed by primary fetch
			if fl.Confidence < 0.95 {
				fl.Confidence = 0.95
			}
			out = append(out, fl)
			continue
		}
		// Path probes and anchors and platform links: confirm when enabled
		needConfirm := c.confirmFeedLinks
		if fl.Source == SourceHTMLLink || fl.Source == SourceHTTPHeader {
			needConfirm = c.confirmHTMLLinks
		}
		if !needConfirm {
			out = append(out, fl)
			continue
		}

		res, err := c.fetchURL(ctx, fl.URL)
		if err != nil {
			warnings = append(warnings, "confirm failed for "+fl.URL+": "+err.Error())
			if !dropInvalid {
				out = append(out, fl)
			}
			continue
		}
		class := ClassifyResponse(res.ContentType, res.StatusCode, res.Body)
		if class.Kind != ContentFeed {
			warnings = append(warnings, "not a feed document: "+fl.URL)
			if !dropInvalid {
				// keep with low confidence
				fl.Confidence = 0.2
				out = append(out, fl)
			}
			continue
		}
		if class.FeedType != "" && class.FeedType != FeedTypeUnknown {
			fl.Type = class.FeedType
		}
		fl.Confidence = 0.95
		fl.URL = res.URL // honor redirects
		out = append(out, fl)
	}
	return out, warnings
}
