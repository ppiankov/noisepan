package taste

import (
	"slices"
	"testing"

	"github.com/ppiankov/noisepan/internal/config"
	"github.com/ppiankov/noisepan/internal/source"
)

func testProfile() *config.TasteProfile {
	return &config.TasteProfile{
		Weights: config.Weights{
			HighSignal: map[string]int{
				"kubernetes": 3,
				"cve":        5,
			},
			LowSignal: map[string]int{
				"webinar": -4,
				"hiring":  -3,
			},
		},
		Labels: map[string][]string{
			"critical": {"cve"},
			"ops":      {"kubernetes"},
		},
		Rules: []config.Rule{
			{
				If:   config.RuleCondition{ContainsAny: []string{"expired", "certificate"}},
				Then: config.RuleAction{ScoreAdd: 4, Labels: []string{"ops", "certs"}},
			},
			{
				If:   config.RuleCondition{ContainsAny: []string{"webinar", "join us"}},
				Then: config.RuleAction{ScoreAdd: -6, Labels: []string{"noise"}},
			},
		},
		Thresholds: config.Thresholds{
			ReadNow: 7,
			Skim:    3,
			Ignore:  0,
		},
	}
}

func post(text string) source.Post {
	return source.Post{Source: "test", Channel: "ch", ExternalID: "1", Text: text}
}

func TestScore_HighSignalOnly(t *testing.T) {
	result := Score(post("kubernetes deployment tips"), testProfile())

	if result.Score != 3 {
		t.Errorf("score = %d, want 3", result.Score)
	}
	if result.Tier != TierSkim {
		t.Errorf("tier = %q, want skim", result.Tier)
	}
}

func TestScore_LowSignalOnly(t *testing.T) {
	result := Score(post("join our webinar today"), testProfile())

	// keyword: webinar (-4) + rule: webinar (-6) = -10
	if result.Score != -10 {
		t.Errorf("score = %d, want -10", result.Score)
	}
	if result.Tier != TierIgnore {
		t.Errorf("tier = %q, want ignore", result.Tier)
	}
}

func TestScore_Accumulation(t *testing.T) {
	result := Score(post("kubernetes CVE-2026-1234 found"), testProfile())

	// kubernetes:3 + cve:5 = 8
	if result.Score != 8 {
		t.Errorf("score = %d, want 8", result.Score)
	}
	if result.Tier != TierReadNow {
		t.Errorf("tier = %q, want read_now", result.Tier)
	}
}

func TestScore_RuleMatching(t *testing.T) {
	result := Score(post("expired certificate rotation needed"), testProfile())

	// rule: expired/certificate → +4
	if result.Score != 4 {
		t.Errorf("score = %d, want 4", result.Score)
	}
	if !slices.Contains(result.Labels, "ops") {
		t.Errorf("labels = %v, want containing ops", result.Labels)
	}
	if !slices.Contains(result.Labels, "certs") {
		t.Errorf("labels = %v, want containing certs", result.Labels)
	}
}

func TestScore_RuleNoMatch(t *testing.T) {
	result := Score(post("simple hello world"), testProfile())

	if result.Score != 0 {
		t.Errorf("score = %d, want 0", result.Score)
	}
	if len(result.Labels) != 0 {
		t.Errorf("labels = %v, want empty", result.Labels)
	}
}

func TestScore_LabelsDeduplicated(t *testing.T) {
	profile := &config.TasteProfile{
		Rules: []config.Rule{
			{
				If:   config.RuleCondition{ContainsAny: []string{"expired"}},
				Then: config.RuleAction{ScoreAdd: 2, Labels: []string{"ops", "certs"}},
			},
			{
				If:   config.RuleCondition{ContainsAny: []string{"certificate"}},
				Then: config.RuleAction{ScoreAdd: 2, Labels: []string{"ops", "tls"}},
			},
		},
		Thresholds: config.Thresholds{ReadNow: 10, Skim: 3, Ignore: 0},
	}

	result := Score(post("expired certificate warning"), profile)

	// Both rules match: ops appears twice, should be deduped
	if !slices.IsSorted(result.Labels) {
		t.Errorf("labels not sorted: %v", result.Labels)
	}
	opsCount := 0
	for _, l := range result.Labels {
		if l == "ops" {
			opsCount++
		}
	}
	if opsCount != 1 {
		t.Errorf("ops appears %d times, want 1", opsCount)
	}
	if len(result.Labels) != 3 {
		t.Errorf("labels = %v, want [certs ops tls]", result.Labels)
	}
}

func TestScore_TierReadNow(t *testing.T) {
	// kubernetes:3 + cve:5 = 8 >= 7 → read_now
	result := Score(post("kubernetes cve alert"), testProfile())

	if result.Tier != TierReadNow {
		t.Errorf("tier = %q, want read_now", result.Tier)
	}
}

func TestScore_TierSkim(t *testing.T) {
	// kubernetes:3, 3 >= 3 → skim
	result := Score(post("kubernetes news"), testProfile())

	if result.Tier != TierSkim {
		t.Errorf("tier = %q, want skim", result.Tier)
	}
}

func TestScore_TierIgnore(t *testing.T) {
	result := Score(post("random unrelated text"), testProfile())

	if result.Tier != TierIgnore {
		t.Errorf("tier = %q, want ignore", result.Tier)
	}
}

func TestScore_EmptyProfile(t *testing.T) {
	profile := &config.TasteProfile{
		Thresholds: config.Thresholds{ReadNow: 7, Skim: 3, Ignore: 0},
	}

	result := Score(post("anything at all"), profile)

	if result.Score != 0 {
		t.Errorf("score = %d, want 0", result.Score)
	}
	if result.Tier != TierIgnore {
		t.Errorf("tier = %q, want ignore", result.Tier)
	}
}

func TestScore_ExplanationCompleteness(t *testing.T) {
	result := Score(post("kubernetes cve expired cert"), testProfile())

	// kubernetes:3 + cve:5 + rule(expired):4 = 12
	if result.Score != 12 {
		t.Errorf("score = %d, want 12", result.Score)
	}

	// Should have 3 explanations: 2 keywords + 1 rule
	if len(result.Explanation) != 3 {
		t.Errorf("explanation count = %d, want 3", len(result.Explanation))
	}

	// Sum of explanation points should equal total score
	pointsSum := 0
	for _, e := range result.Explanation {
		pointsSum += e.Points
	}
	if pointsSum != result.Score {
		t.Errorf("explanation points sum = %d, score = %d", pointsSum, result.Score)
	}
}

func TestScore_CaseInsensitive(t *testing.T) {
	result := Score(post("KUBERNETES CLUSTER UPDATE"), testProfile())

	if result.Score != 3 {
		t.Errorf("score = %d, want 3", result.Score)
	}
}
