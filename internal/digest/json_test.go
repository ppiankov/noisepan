package digest

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/ppiankov/noisepan/internal/source"
	"github.com/ppiankov/noisepan/internal/summarize"
	"github.com/ppiankov/noisepan/internal/taste"
)

func TestJSONFormat_Full(t *testing.T) {
	input := DigestInput{
		Items: []DigestItem{
			{
				ScoredPost: taste.ScoredPost{
					Post:   source.Post{Source: "rss", Channel: "blog", URL: "https://example.com/1", PostedAt: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)},
					Score:  9,
					Tier:   taste.TierReadNow,
					Labels: []string{"critical"},
				},
				Summary: summarize.Summary{Bullets: []string{"CVE found", "Affects v2.0"}},
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
		Since:      24 * time.Hour,
	}

	var buf bytes.Buffer
	f := NewJSON()
	if err := f.Format(&buf, input); err != nil {
		t.Fatalf("format: %v", err)
	}

	var result jsonDigest
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, buf.String())
	}

	if result.Meta.Channels != 3 {
		t.Errorf("channels = %d, want 3", result.Meta.Channels)
	}
	if result.Meta.TotalPosts != 10 {
		t.Errorf("total_posts = %d, want 10", result.Meta.TotalPosts)
	}
	if result.Meta.Since != "1d" {
		t.Errorf("since = %q, want 1d", result.Meta.Since)
	}
	if len(result.ReadNow) != 1 {
		t.Fatalf("read_now count = %d, want 1", len(result.ReadNow))
	}
	if result.ReadNow[0].Headline != "CVE found" {
		t.Errorf("headline = %q, want CVE found", result.ReadNow[0].Headline)
	}
	if len(result.ReadNow[0].Bullets) != 1 || result.ReadNow[0].Bullets[0] != "Affects v2.0" {
		t.Errorf("bullets = %v, want [Affects v2.0]", result.ReadNow[0].Bullets)
	}
	if len(result.ReadNow[0].AlsoIn) != 1 {
		t.Errorf("also_in = %v, want [telegram/@sec]", result.ReadNow[0].AlsoIn)
	}
	if len(result.Skims) != 1 {
		t.Fatalf("skims count = %d, want 1", len(result.Skims))
	}
	if result.Skims[0].Channel != "devops" {
		t.Errorf("skim channel = %q, want devops", result.Skims[0].Channel)
	}
	if result.Ignored != 1 {
		t.Errorf("ignored = %d, want 1", result.Ignored)
	}
}

func TestJSONFormat_Empty(t *testing.T) {
	input := DigestInput{
		Channels:   0,
		TotalPosts: 0,
		Since:      24 * time.Hour,
	}

	var buf bytes.Buffer
	f := NewJSON()
	if err := f.Format(&buf, input); err != nil {
		t.Fatalf("format: %v", err)
	}

	var result jsonDigest
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(result.ReadNow) != 0 {
		t.Errorf("read_now = %v, want empty", result.ReadNow)
	}
	if len(result.Skims) != 0 {
		t.Errorf("skims = %v, want empty", result.Skims)
	}
	if result.Ignored != 0 {
		t.Errorf("ignored = %d, want 0", result.Ignored)
	}
}

func TestJSONFormat_Omitempty(t *testing.T) {
	input := DigestInput{
		Items: []DigestItem{
			{
				ScoredPost: taste.ScoredPost{
					Post:  source.Post{Source: "rss", Channel: "ch", PostedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
					Score: 5,
					Tier:  taste.TierSkim,
				},
				Summary: summarize.Summary{Bullets: []string{"only headline"}},
			},
		},
		Channels:   1,
		TotalPosts: 1,
		Since:      24 * time.Hour,
	}

	var buf bytes.Buffer
	f := NewJSON()
	if err := f.Format(&buf, input); err != nil {
		t.Fatalf("format: %v", err)
	}

	// Check raw JSON for absence of omitempty fields
	raw := buf.String()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	skims := m["skims"].([]any)
	item := skims[0].(map[string]any)
	if _, ok := item["url"]; ok {
		t.Error("url should be omitted when empty")
	}
	if _, ok := item["labels"]; ok {
		t.Error("labels should be omitted when nil")
	}
	if _, ok := item["also_in"]; ok {
		t.Error("also_in should be omitted when nil")
	}
	if _, ok := item["bullets"]; ok {
		t.Error("bullets should be omitted when empty")
	}
}
