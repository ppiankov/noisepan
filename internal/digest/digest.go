package digest

import (
	"io"
	"time"

	"github.com/ppiankov/noisepan/internal/summarize"
	"github.com/ppiankov/noisepan/internal/taste"
)

// DigestItem pairs a scored post with its summary.
type DigestItem struct {
	taste.ScoredPost
	Summary summarize.Summary
}

// DigestInput is the full input for a digest formatter.
type DigestInput struct {
	Items      []DigestItem
	Channels   int           // number of channels fetched
	TotalPosts int           // total posts before filtering
	Since      time.Duration // time window
}

// Formatter writes a formatted digest to w.
type Formatter interface {
	Format(w io.Writer, input DigestInput) error
}
