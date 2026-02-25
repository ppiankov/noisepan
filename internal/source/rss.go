package source

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
)

const (
	rssSourceName   = "rss"
	rssFetchTimeout = 30 * time.Second
	rssUserAgent    = "Mozilla/5.0 (compatible; noisepan/1.0; +https://github.com/ppiankov/noisepan)"
	rssMaxWorkers   = 10
	rssMaxRetries   = 3
	rssDomainDelay  = 3 * time.Second
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
	type result struct {
		posts []Post
		err   error
		url   string
	}

	// Group feeds by domain so same-domain requests are serialized.
	domainFeeds := make(map[string][]string)
	for _, feedURL := range rs.feeds {
		d := feedDomain(feedURL)
		domainFeeds[d] = append(domainFeeds[d], feedURL)
	}

	results := make(chan result, len(rs.feeds))
	domainJobs := make(chan []string, len(domainFeeds))

	workers := rssMaxWorkers
	if len(domainFeeds) < workers {
		workers = len(domainFeeds)
	}

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for feeds := range domainJobs {
				for i, feedURL := range feeds {
					if i > 0 {
						rssSleepFunc(rssDomainDelay)
					}
					items, err := fetchWithRetry(feedURL, since)
					results <- result{posts: items, err: err, url: feedURL}
				}
			}
		}()
	}

	for _, feeds := range domainFeeds {
		domainJobs <- feeds
	}
	close(domainJobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	var posts []Post
	for r := range results {
		if r.err != nil {
			fmt.Printf("  rss: %s: %v\n", r.url, r.err)
			continue
		}
		posts = append(posts, r.posts...)
	}

	return posts, nil
}

// feedDomain extracts the host from a feed URL for rate limiting grouping.
func feedDomain(feedURL string) string {
	u, err := url.Parse(feedURL)
	if err != nil || u.Host == "" {
		return feedURL
	}
	return u.Host
}

// rssTransport injects a User-Agent header into every request.
type rssTransport struct {
	base http.RoundTripper
}

func (t *rssTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", rssUserAgent)
	return t.base.RoundTrip(req)
}

// rssSleepFunc is the function used for retry backoff delays.
// It defaults to time.Sleep but can be overridden in tests.
var rssSleepFunc = time.Sleep

func fetchWithRetry(feedURL string, since time.Time) ([]Post, error) {
	var lastErr error
	for attempt := range rssMaxRetries {
		posts, err := fetchFeed(feedURL, since)
		if err == nil {
			return posts, nil
		}
		if !isRetryableError(err) {
			return nil, err
		}
		lastErr = err
		if attempt < rssMaxRetries-1 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second // 1s, 2s, 4s
			rssSleepFunc(backoff)
		}
	}
	return nil, lastErr
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	// Timeout errors
	if strings.Contains(s, "timeout") || strings.Contains(s, "Timeout") {
		return true
	}
	// Connection errors
	if strings.Contains(s, "connection refused") || strings.Contains(s, "no such host") {
		return true
	}
	// HTTP 5xx errors (server-side, worth retrying)
	if strings.Contains(s, "500") ||
		strings.Contains(s, "502") || strings.Contains(s, "503") || strings.Contains(s, "504") {
		return true
	}
	return false
}

func fetchFeed(feedURL string, since time.Time) ([]Post, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rssFetchTimeout)
	defer cancel()

	fp := gofeed.NewParser()
	fp.Client = &http.Client{
		Timeout:   rssFetchTimeout,
		Transport: &rssTransport{base: http.DefaultTransport},
	}
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
