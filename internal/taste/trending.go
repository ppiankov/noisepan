package taste

import (
	"sort"
	"strings"

	"github.com/ppiankov/noisepan/internal/config"
)

// Trend represents a keyword or URL seen across multiple channels.
type Trend struct {
	Keyword  string   // the keyword or URL that trended
	Channels []string // distinct channel names
}

// FindTrending detects high-signal keywords appearing in minSources or more distinct channels.
// Only considers keywords from the taste profile (high_signal + rule contains_any).
func FindTrending(posts []ScoredPost, profile *config.TasteProfile, minSources int) []Trend {
	if minSources < 2 {
		minSources = 2
	}

	// Collect all signal keywords to check
	keywords := collectKeywords(profile)
	if len(keywords) == 0 {
		return nil
	}

	// Also track shared URLs
	urlChannels := make(map[string]map[string]bool)

	// For each keyword, track which channels mention it
	kwChannels := make(map[string]map[string]bool)

	for _, sp := range posts {
		if sp.Tier != TierReadNow && sp.Tier != TierSkim {
			continue
		}
		textLower := strings.ToLower(sp.Post.Text)

		for _, kw := range keywords {
			if strings.Contains(textLower, strings.ToLower(kw)) {
				if kwChannels[kw] == nil {
					kwChannels[kw] = make(map[string]bool)
				}
				kwChannels[kw][sp.Post.Channel] = true
			}
		}

		if sp.Post.URL != "" {
			if urlChannels[sp.Post.URL] == nil {
				urlChannels[sp.Post.URL] = make(map[string]bool)
			}
			urlChannels[sp.Post.URL][sp.Post.Channel] = true
		}
	}

	// Build trends for keywords meeting threshold
	seen := make(map[string]bool) // avoid duplicate channels from kw + url overlap
	var trends []Trend

	for kw, chMap := range kwChannels {
		if len(chMap) < minSources {
			continue
		}
		channels := mapKeys(chMap)
		sort.Strings(channels)
		trends = append(trends, Trend{Keyword: kw, Channels: channels})
		seen[kw] = true
	}

	for url, chMap := range urlChannels {
		if len(chMap) < minSources || seen[url] {
			continue
		}
		channels := mapKeys(chMap)
		sort.Strings(channels)
		trends = append(trends, Trend{Keyword: url, Channels: channels})
	}

	// Sort by channel count descending, then keyword alphabetically
	sort.Slice(trends, func(i, j int) bool {
		if len(trends[i].Channels) != len(trends[j].Channels) {
			return len(trends[i].Channels) > len(trends[j].Channels)
		}
		return trends[i].Keyword < trends[j].Keyword
	})

	return trends
}

func collectKeywords(profile *config.TasteProfile) []string {
	seen := make(map[string]bool)
	var keywords []string

	for kw := range profile.Weights.HighSignal {
		lower := strings.ToLower(kw)
		if !seen[lower] {
			keywords = append(keywords, kw)
			seen[lower] = true
		}
	}

	for _, rule := range profile.Rules {
		for _, kw := range rule.If.ContainsAny {
			lower := strings.ToLower(kw)
			if !seen[lower] {
				keywords = append(keywords, kw)
				seen[lower] = true
			}
		}
	}

	return keywords
}

func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
