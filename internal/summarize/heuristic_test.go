package summarize

import (
	"strings"
	"testing"
)

func TestSummarize_BasicText(t *testing.T) {
	s := &HeuristicSummarizer{}
	result := s.Summarize("Kubernetes 1.32 has been released with new features.")

	if len(result.Bullets) < 1 {
		t.Fatal("expected at least 1 bullet")
	}
	if !strings.Contains(result.Bullets[0], "Kubernetes") {
		t.Errorf("bullet[0] = %q, want containing Kubernetes", result.Bullets[0])
	}
}

func TestSummarize_WithURLs(t *testing.T) {
	s := &HeuristicSummarizer{}
	result := s.Summarize("Check https://example.com/1 and https://example.com/2 for details.")

	if len(result.Links) != 2 {
		t.Errorf("links count = %d, want 2", len(result.Links))
	}
	if result.Links[0] != "https://example.com/1" {
		t.Errorf("links[0] = %q", result.Links[0])
	}
}

func TestSummarize_WithCVEs(t *testing.T) {
	s := &HeuristicSummarizer{}
	result := s.Summarize("Critical vulnerability CVE-2026-1234 found in libfoo. Update immediately.")

	if len(result.CVEs) != 1 || result.CVEs[0] != "CVE-2026-1234" {
		t.Errorf("cves = %v, want [CVE-2026-1234]", result.CVEs)
	}

	// Should have a CVE bullet
	hasCVEBullet := false
	for _, b := range result.Bullets {
		if strings.Contains(b, "CVE-2026-1234") {
			hasCVEBullet = true
			break
		}
	}
	if !hasCVEBullet {
		t.Errorf("bullets = %v, want one containing CVE", result.Bullets)
	}
}

func TestSummarize_WithVersions(t *testing.T) {
	s := &HeuristicSummarizer{}
	result := s.Summarize("Upgrade from v1.2.3 to v1.3.0 for the fix.")

	hasVersionBullet := false
	for _, b := range result.Bullets {
		if strings.Contains(b, "v1.2.3") || strings.Contains(b, "Versions:") {
			hasVersionBullet = true
			break
		}
	}
	if !hasVersionBullet {
		t.Errorf("bullets = %v, want one mentioning versions", result.Bullets)
	}
}

func TestSummarize_BreakingChange(t *testing.T) {
	s := &HeuristicSummarizer{}
	text := "Minor update released. This is a breaking change in the API. Migrate before v2."
	result := s.Summarize(text)

	hasBreaking := false
	for _, b := range result.Bullets {
		if strings.Contains(b, "breaking change") {
			hasBreaking = true
			break
		}
	}
	if !hasBreaking {
		t.Errorf("bullets = %v, want one containing 'breaking change'", result.Bullets)
	}
}

func TestSummarize_Deprecated(t *testing.T) {
	s := &HeuristicSummarizer{}
	text := "New release available. The old API is deprecated and will be removed in v3."
	result := s.Summarize(text)

	hasDeprecated := false
	for _, b := range result.Bullets {
		if strings.Contains(b, "deprecated") {
			hasDeprecated = true
			break
		}
	}
	if !hasDeprecated {
		t.Errorf("bullets = %v, want one containing 'deprecated'", result.Bullets)
	}
}

func TestSummarize_ManyURLs(t *testing.T) {
	s := &HeuristicSummarizer{}
	text := "Links: https://a.com https://b.com https://c.com https://d.com https://e.com"
	result := s.Summarize(text)

	if len(result.Links) != 5 {
		t.Errorf("links count = %d, want 5", len(result.Links))
	}

	hasLinksBullet := false
	for _, b := range result.Bullets {
		if strings.Contains(b, "5 links included") {
			hasLinksBullet = true
			break
		}
	}
	if !hasLinksBullet {
		t.Errorf("bullets = %v, want one containing '5 links included'", result.Bullets)
	}
}

func TestSummarize_EmptyText(t *testing.T) {
	s := &HeuristicSummarizer{}
	result := s.Summarize("")

	if len(result.Bullets) < 1 {
		t.Fatal("expected at least 1 bullet")
	}
	if result.Bullets[0] != "(empty)" {
		t.Errorf("bullet[0] = %q, want (empty)", result.Bullets[0])
	}
}

func TestSummarize_ShortText(t *testing.T) {
	s := &HeuristicSummarizer{}
	result := s.Summarize("Short message")

	if result.Bullets[0] != "Short message" {
		t.Errorf("bullet[0] = %q, want 'Short message'", result.Bullets[0])
	}
}

func TestSummarize_LongFirstSentence(t *testing.T) {
	s := &HeuristicSummarizer{}
	long := strings.Repeat("word ", 30) + "end of sentence."
	result := s.Summarize(long)

	if len(result.Bullets[0]) > 130 { // 120 + "..."
		t.Errorf("bullet[0] length = %d, want <= 130", len(result.Bullets[0]))
	}
	if !strings.HasSuffix(result.Bullets[0], "...") {
		t.Errorf("bullet[0] = %q, want ending with ...", result.Bullets[0])
	}
}

func TestSummarize_MaxBullets(t *testing.T) {
	s := &HeuristicSummarizer{}
	result := s.Summarize("First sentence. This is a breaking change. CVE-2026-9999 found. Deprecated API.")

	if len(result.Bullets) > 3 {
		t.Errorf("bullets count = %d, want <= 3", len(result.Bullets))
	}
}
