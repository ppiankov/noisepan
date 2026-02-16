package summarize

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	urlRe     = regexp.MustCompile(`https?://\S+`)
	cveRe     = regexp.MustCompile(`CVE-\d{4}-\d{4,}`)
	versionRe = regexp.MustCompile(`v?\d+\.\d+\.\d+`)
)

const (
	maxBullets       = 3
	maxFirstSentence = 120
)

var alertKeywords = []string{"breaking change", "deprecated", "removed"}

// HeuristicSummarizer summarizes text using rule-based extraction.
type HeuristicSummarizer struct{}

// Summarize extracts key points, URLs, and CVE IDs from text.
func (h *HeuristicSummarizer) Summarize(text string) Summary {
	text = strings.TrimSpace(text)

	links := urlRe.FindAllString(text, -1)
	cves := cveRe.FindAllString(text, -1)
	versions := versionRe.FindAllString(text, -1)

	var bullets []string

	// Bullet 1: first sentence (always present)
	first := firstSentence(text, maxFirstSentence)
	if first == "" {
		first = "(empty)"
	}
	bullets = append(bullets, first)

	// Bullet 2: sentence containing alert keywords
	if sent := findSentenceContaining(text, alertKeywords); sent != "" && sent != first {
		bullets = append(bullets, sent)
	}

	// Bullet 3: metadata summary
	if len(bullets) < maxBullets {
		if len(cves) > 0 {
			bullets = append(bullets, "CVE: "+strings.Join(cves, ", "))
		} else if len(versions) > 0 {
			bullets = append(bullets, "Versions: "+strings.Join(versions, ", "))
		} else if len(links) > 3 {
			bullets = append(bullets, fmt.Sprintf("%d links included", len(links)))
		}
	}

	// Cap at max
	if len(bullets) > maxBullets {
		bullets = bullets[:maxBullets]
	}

	return Summary{
		Bullets: bullets,
		Links:   links,
		CVEs:    cves,
	}
}

// firstSentence returns text up to the first sentence boundary, capped at maxLen.
func firstSentence(text string, maxLen int) string {
	if text == "" {
		return ""
	}

	// Find first newline
	end := len(text)
	if idx := strings.IndexByte(text, '\n'); idx >= 0 && idx < end {
		end = idx
	}

	// Find first ". " or ".\n" (period followed by space or newline)
	for i := 0; i < end-1; i++ {
		if text[i] == '.' && (text[i+1] == ' ' || text[i+1] == '\n') {
			end = i + 1 // include the period
			break
		}
	}
	if end > maxLen {
		// Truncate at last space before maxLen to avoid cutting words
		if idx := strings.LastIndexByte(text[:maxLen], ' '); idx > 0 {
			return text[:idx] + "..."
		}
		return text[:maxLen] + "..."
	}

	return strings.TrimSpace(text[:end])
}

// findSentenceContaining returns the first sentence that contains any keyword.
func findSentenceContaining(text string, keywords []string) string {
	textLower := strings.ToLower(text)
	sentences := splitSentences(text)

	for i, sent := range sentences {
		// Use the lowercase version of the sentence range for matching
		sentLower := strings.ToLower(sent)
		_ = textLower // match against individual sentence
		for _, kw := range keywords {
			if strings.Contains(sentLower, kw) {
				s := strings.TrimSpace(sentences[i])
				if len(s) > maxFirstSentence {
					s = s[:maxFirstSentence] + "..."
				}
				return s
			}
		}
	}
	return ""
}

// splitSentences splits text into sentences by ". " or newline boundaries.
func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	for i := 0; i < len(text); i++ {
		current.WriteByte(text[i])

		if text[i] == '\n' {
			if s := strings.TrimSpace(current.String()); s != "" {
				sentences = append(sentences, s)
			}
			current.Reset()
			continue
		}

		if text[i] == '.' && i+1 < len(text) && (text[i+1] == ' ' || text[i+1] == '\n') {
			if s := strings.TrimSpace(current.String()); s != "" {
				sentences = append(sentences, s)
			}
			current.Reset()
			continue
		}
	}

	if s := strings.TrimSpace(current.String()); s != "" {
		sentences = append(sentences, s)
	}

	return sentences
}
