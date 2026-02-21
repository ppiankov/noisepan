package taste

import (
	"testing"
	"time"

	"github.com/ppiankov/noisepan/internal/config"
	"github.com/ppiankov/noisepan/internal/source"
)

func makePost(channel, text, url string) ScoredPost {
	return ScoredPost{
		Post: source.Post{
			Source:   "rss",
			Channel:  channel,
			Text:     text,
			URL:      url,
			PostedAt: time.Now(),
		},
		Score: 8,
		Tier:  TierReadNow,
	}
}

func TestFindTrending_KeywordAcrossChannels(t *testing.T) {
	posts := []ScoredPost{
		makePost("CISA", "New CVE-2026-1234 vulnerability discovered", "https://cisa.gov/1"),
		makePost("Krebs", "CVE-2026-1234 actively exploited in the wild", "https://krebs.com/1"),
		makePost("BleepingComputer", "CVE-2026-1234 patch available from Microsoft", "https://bleeping.com/1"),
	}

	trends := FindTrending(posts, testProfile(), 3)

	if len(trends) == 0 {
		t.Fatal("expected at least one trend")
	}

	// CVE- or cve should be trending (appears in 3 channels)
	found := false
	for _, tr := range trends {
		if len(tr.Channels) >= 3 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a trend with 3+ channels, got: %+v", trends)
	}
}

func TestFindTrending_BelowThreshold(t *testing.T) {
	posts := []ScoredPost{
		makePost("CISA", "New CVE found", ""),
		makePost("Krebs", "CVE report published", ""),
	}

	trends := FindTrending(posts, testProfile(), 3)

	if len(trends) != 0 {
		t.Errorf("expected 0 trends (only 2 channels), got %d", len(trends))
	}
}

func TestFindTrending_IgnoredPostsExcluded(t *testing.T) {
	posts := []ScoredPost{
		{Post: source.Post{Channel: "a", Text: "cve stuff"}, Tier: TierIgnore, Score: 1},
		{Post: source.Post{Channel: "b", Text: "cve stuff"}, Tier: TierIgnore, Score: 1},
		{Post: source.Post{Channel: "c", Text: "cve stuff"}, Tier: TierIgnore, Score: 1},
	}

	trends := FindTrending(posts, testProfile(), 3)

	if len(trends) != 0 {
		t.Errorf("expected 0 trends (all ignored), got %d", len(trends))
	}
}

func TestFindTrending_SharedURL(t *testing.T) {
	url := "https://example.com/article"
	posts := []ScoredPost{
		makePost("feed-a", "some article about kubernetes", url),
		makePost("feed-b", "shared article about kubernetes", url),
		makePost("feed-c", "kubernetes article link", url),
	}

	trends := FindTrending(posts, testProfile(), 3)

	if len(trends) == 0 {
		t.Fatal("expected at least one trend")
	}
}

func TestFindTrending_EmptyPosts(t *testing.T) {
	trends := FindTrending(nil, testProfile(), 3)
	if len(trends) != 0 {
		t.Errorf("expected 0 trends for nil posts, got %d", len(trends))
	}
}

func TestFindTrending_EmptyProfile(t *testing.T) {
	posts := []ScoredPost{makePost("a", "hello", "")}
	profile := &config.TasteProfile{Thresholds: config.Thresholds{ReadNow: 7, Skim: 3, Ignore: 0}}

	trends := FindTrending(posts, profile, 3)
	if len(trends) != 0 {
		t.Errorf("expected 0 trends for empty profile, got %d", len(trends))
	}
}

func TestFindTrending_SortedByChannelCount(t *testing.T) {
	posts := []ScoredPost{
		makePost("a", "kubernetes update", ""),
		makePost("b", "kubernetes news", ""),
		makePost("c", "kubernetes release", ""),
		makePost("a", "cve found zero-day", ""),
		makePost("b", "cve found zero-day", ""),
		makePost("c", "cve found zero-day", ""),
		makePost("d", "cve found zero-day", ""),
	}

	trends := FindTrending(posts, testProfile(), 3)

	if len(trends) < 2 {
		t.Fatalf("expected at least 2 trends, got %d", len(trends))
	}

	// First trend should have more channels than second
	if len(trends[0].Channels) < len(trends[1].Channels) {
		t.Errorf("trends not sorted by channel count: %d < %d",
			len(trends[0].Channels), len(trends[1].Channels))
	}
}
