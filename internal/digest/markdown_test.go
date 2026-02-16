package digest

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ppiankov/noisepan/internal/source"
	"github.com/ppiankov/noisepan/internal/summarize"
	"github.com/ppiankov/noisepan/internal/taste"
)

func TestMarkdownFormat_Full(t *testing.T) {
	input := DigestInput{
		Items: []DigestItem{
			{
				ScoredPost: taste.ScoredPost{
					Post:   source.Post{Source: "rss", Channel: "blog", URL: "https://example.com/1", PostedAt: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)},
					Score:  9,
					Tier:   taste.TierReadNow,
					Labels: []string{"critical", "ops"},
				},
				Summary: summarize.Summary{Bullets: []string{"CVE found", "Affects v2.0", "Patch available"}},
				AlsoIn:  []string{"telegram/@sec"},
			},
			{
				ScoredPost: taste.ScoredPost{
					Post:  source.Post{Source: "reddit", Channel: "devops", PostedAt: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)},
					Score: 4,
					Tier:  taste.TierSkim,
				},
				Summary: summarize.Summary{Bullets: []string{"K8s update"}},
			},
			{
				ScoredPost: taste.ScoredPost{
					Post:  source.Post{Source: "rss", Channel: "noise", PostedAt: time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC)},
					Score: 1,
					Tier:  taste.TierIgnore,
				},
				Summary: summarize.Summary{Bullets: []string{"Ad"}},
			},
		},
		Channels:   3,
		TotalPosts: 10,
		Since:      48 * time.Hour,
	}

	var buf bytes.Buffer
	f := NewMarkdown()
	if err := f.Format(&buf, input); err != nil {
		t.Fatalf("format: %v", err)
	}

	out := buf.String()

	checks := []string{
		"# noisepan digest",
		"3 channels, 10 posts, since 2d",
		"## Read Now (1)",
		"### [9] blog — CVE found",
		"`critical` `ops`",
		"- Affects v2.0",
		"- Patch available",
		"Also in: telegram/@sec",
		"[Link](https://example.com/1)",
		"## Skim (1)",
		"- **[4]** devops — K8s update",
		"*Ignored: 1 posts*",
	}

	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n\nfull output:\n%s", want, out)
		}
	}
}

func TestMarkdownFormat_Empty(t *testing.T) {
	input := DigestInput{
		Channels:   0,
		TotalPosts: 0,
		Since:      24 * time.Hour,
	}

	var buf bytes.Buffer
	f := NewMarkdown()
	if err := f.Format(&buf, input); err != nil {
		t.Fatalf("format: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "# noisepan digest") {
		t.Error("missing header")
	}
	if !strings.Contains(out, "No posts found.") {
		t.Error("missing 'No posts found.'")
	}
}

func TestMarkdownFormat_URLRendering(t *testing.T) {
	input := DigestInput{
		Items: []DigestItem{
			{
				ScoredPost: taste.ScoredPost{
					Post:  source.Post{Source: "rss", Channel: "blog", URL: "https://example.com/post", PostedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
					Score: 8,
					Tier:  taste.TierReadNow,
				},
				Summary: summarize.Summary{Bullets: []string{"Headline"}},
			},
			{
				ScoredPost: taste.ScoredPost{
					Post:  source.Post{Source: "rss", Channel: "other", PostedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
					Score: 8,
					Tier:  taste.TierReadNow,
				},
				Summary: summarize.Summary{Bullets: []string{"No URL"}},
			},
		},
		Channels:   2,
		TotalPosts: 2,
		Since:      24 * time.Hour,
	}

	var buf bytes.Buffer
	f := NewMarkdown()
	if err := f.Format(&buf, input); err != nil {
		t.Fatalf("format: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[Link](https://example.com/post)") {
		t.Errorf("missing link for post with URL\n\nfull output:\n%s", out)
	}
	// Post without URL should not have a Link line
	lines := strings.Split(out, "\n")
	linkCount := 0
	for _, line := range lines {
		if strings.Contains(line, "[Link]") {
			linkCount++
		}
	}
	if linkCount != 1 {
		t.Errorf("link count = %d, want 1 (only for post with URL)", linkCount)
	}
}
