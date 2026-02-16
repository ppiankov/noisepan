package source

import (
	"context"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

const (
	rssSourceName   = "rss"
	rssFetchTimeout = 30 * time.Second
)

var (
	htmlTagRe    = regexp.MustCompile(`<[^>]*>`)
	whitespaceRe = regexp.MustCompile(`\s{3,}`)
)

// RSSSource fetches posts from RSS/Atom feeds.
type RSSSource struct {
	feeds []string
}

// NewRSS creates an RSS/Atom source. At least one feed URL is required.
func NewRSS(feeds []string) (*RSSSource, error) {
	if len(feeds) == 0 {
		return nil, errors.New("rss: at least one feed URL is required")
	}
	return &RSSSource{feeds: feeds}, nil
}

func (rs *RSSSource) Name() string {
	return rssSourceName
}

func (rs *RSSSource) Fetch(since time.Time) ([]Post, error) {
	var posts []Post

	for _, feedURL := range rs.feeds {
		items, err := fetchFeed(feedURL, since)
		if err != nil {
			fmt.Printf("  rss: %s: %v\n", feedURL, err)
			continue
		}
		posts = append(posts, items...)
	}

	return posts, nil
}

func fetchFeed(feedURL string, since time.Time) ([]Post, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rssFetchTimeout)
	defer cancel()

	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(feedURL, ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", feedURL, err)
	}

	return postsFromFeed(feed, feedURL, since), nil
}

func postsFromFeed(feed *gofeed.Feed, feedURL string, since time.Time) []Post {
	var posts []Post
	for _, item := range feed.Items {
		postedAt := itemPublishedTime(item)
		if postedAt.IsZero() || postedAt.Before(since) {
			continue
		}

		posts = append(posts, Post{
			Source:     rssSourceName,
			Channel:    feedLabel(feed, feedURL),
			ExternalID: itemID(item),
			Text:       itemText(item),
			URL:        item.Link,
			PostedAt:   postedAt,
		})
	}
	return posts
}

func itemPublishedTime(item *gofeed.Item) time.Time {
	if item.PublishedParsed != nil {
		return *item.PublishedParsed
	}
	if item.UpdatedParsed != nil {
		return *item.UpdatedParsed
	}
	return time.Time{}
}

func feedLabel(feed *gofeed.Feed, feedURL string) string {
	if feed.Title != "" {
		return feed.Title
	}
	return feedURL
}

func itemID(item *gofeed.Item) string {
	if item.GUID != "" {
		return item.GUID
	}
	return item.Link
}

func itemText(item *gofeed.Item) string {
	raw := item.Content
	if raw == "" {
		raw = item.Description
	}

	text := stripHTML(raw)

	if item.Title != "" && !strings.Contains(text, item.Title) {
		text = item.Title + "\n\n" + text
	}

	return strings.TrimSpace(text)
}

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = whitespaceRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
