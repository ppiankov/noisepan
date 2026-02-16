package source

import "time"

// Post represents a single item fetched from an information source.
type Post struct {
	Source     string    // source identifier: "telegram", "rss", "reddit"
	Channel    string    // channel/feed/subreddit name
	ExternalID string    // source-specific unique ID
	Text       string    // full message text
	URL        string    // link to the original item
	PostedAt   time.Time // publication timestamp
}

// Source fetches posts from an information stream.
type Source interface {
	// Name returns the source identifier (e.g. "telegram").
	Name() string

	// Fetch returns posts published after the given time.
	Fetch(since time.Time) ([]Post, error)
}
