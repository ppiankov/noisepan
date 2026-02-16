package summarize

// Summary holds the result of summarizing a post's text.
type Summary struct {
	Bullets []string // 1-3 key points
	Links   []string // extracted URLs
	CVEs    []string // extracted CVE IDs
}

// Summarizer produces a summary from post text.
type Summarizer interface {
	Summarize(text string) Summary
}
