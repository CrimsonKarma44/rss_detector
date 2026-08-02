package rssdetector

import "sort"

// dedupeAndRank merges feed links by canonical URL, keeping the best entry.
func dedupeAndRank(links []FeedLink) []FeedLink {
	if len(links) == 0 {
		return nil
	}
	best := make(map[string]FeedLink, len(links))
	order := make([]string, 0, len(links))
	for _, fl := range links {
		key := CanonicalFeedURL(fl.URL)
		if key == "" {
			continue
		}
		fl.URL = key
		prev, ok := best[key]
		if !ok {
			best[key] = fl
			order = append(order, key)
			continue
		}
		if score(fl) > score(prev) {
			best[key] = fl
		} else if score(fl) == score(prev) && fl.Title != "" && prev.Title == "" {
			best[key] = fl
		}
	}
	out := make([]FeedLink, 0, len(best))
	for _, k := range order {
		if fl, ok := best[k]; ok {
			out = append(out, fl)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return score(out[i]) > score(out[j])
	})
	return out
}

func score(fl FeedLink) float64 {
	s := fl.Confidence
	switch {
	case fl.Source == SourceDirectFeed:
		s += 2.0
	case len(fl.Source) > 9 && fl.Source[:9] == "platform:":
		s += 1.5
	case fl.Source == SourceHTMLLink:
		s += 1.2
	case fl.Source == SourceHTTPHeader:
		s += 1.0
	case fl.Source == SourceCommonPath:
		s += 0.6
	case fl.Source == SourceAnchor:
		s += 0.3
	}
	switch fl.Type {
	case FeedTypeRSS, FeedTypeAtom:
		s += 0.2
	case FeedTypeJSON:
		s += 0.1
	}
	return s
}
