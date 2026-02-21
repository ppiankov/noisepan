package digest

import (
	"io"
	"time"

	"github.com/ppiankov/noisepan/internal/summarize"
	"github.com/ppiankov/noisepan/internal/taste"
)

// Trend is re-exported from taste for convenience in formatters.
type Trend = taste.Trend

// DigestItem pairs a scored post with its summary.
type DigestItem struct {
	taste.ScoredPost
	Summary summarize.Summary
	AlsoIn  []string
}

// DigestInput is the full input for a digest formatter.
type DigestInput struct {
	Items      []DigestItem
	Trending   []Trend       // topics appearing in 3+ channels
	Channels   int           // number of channels fetched
	TotalPosts int           // total posts before filtering
	Since      time.Duration // time window
}

// Formatter writes a formatted digest to w.
type Formatter interface {
	Format(w io.Writer, input DigestInput) error
}
