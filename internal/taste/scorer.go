package taste

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ppiankov/noisepan/internal/config"
	"github.com/ppiankov/noisepan/internal/source"
)

const (
	TierReadNow = "read_now"
	TierSkim    = "skim"
	TierIgnore  = "ignore"
)

// ScoredPost is a post with its computed score, labels, tier, and explanation.
type ScoredPost struct {
	Post        source.Post
	Score       int
	Labels      []string
	Tier        string // "read_now", "skim", "ignore"
	Explanation []ScoreContribution
}

// ScoreContribution records a single scoring reason and its point value.
type ScoreContribution struct {
	Reason string // "keyword: kubernetes" or "rule: contains cve"
	Points int
}

// Score evaluates a post against a taste profile and returns a scored result.
func Score(post source.Post, profile *config.TasteProfile) ScoredPost {
	textLower := strings.ToLower(post.Text)

	var (
		total       int
		labels      []string
		explanation []ScoreContribution
	)

	// High signal keywords
	for kw, weight := range profile.Weights.HighSignal {
		if strings.Contains(textLower, strings.ToLower(kw)) {
			total += weight
			explanation = append(explanation, ScoreContribution{
				Reason: fmt.Sprintf("keyword: %s", kw),
				Points: weight,
			})
		}
	}

	// Low signal keywords
	for kw, weight := range profile.Weights.LowSignal {
		if strings.Contains(textLower, strings.ToLower(kw)) {
			total += weight
			explanation = append(explanation, ScoreContribution{
				Reason: fmt.Sprintf("keyword: %s", kw),
				Points: weight,
			})
		}
	}

	// Rules
	for _, rule := range profile.Rules {
		if ruleMatches(textLower, rule.If) {
			total += rule.Then.ScoreAdd
			labels = append(labels, rule.Then.Labels...)
			reason := "rule"
			if len(rule.If.ContainsAny) > 0 {
				reason = fmt.Sprintf("rule: %s", rule.If.ContainsAny[0])
			}
			explanation = append(explanation, ScoreContribution{
				Reason: reason,
				Points: rule.Then.ScoreAdd,
			})
		}
	}

	// Deduplicate and sort labels
	slices.Sort(labels)
	labels = slices.Compact(labels)

	return ScoredPost{
		Post:        post,
		Score:       total,
		Labels:      labels,
		Tier:        assignTier(total, profile.Thresholds),
		Explanation: explanation,
	}
}

func ruleMatches(textLower string, cond config.RuleCondition) bool {
	for _, kw := range cond.ContainsAny {
		if strings.Contains(textLower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

func assignTier(score int, t config.Thresholds) string {
	if score >= t.ReadNow {
		return TierReadNow
	}
	if score >= t.Skim {
		return TierSkim
	}
	return TierIgnore
}
