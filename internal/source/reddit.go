package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	redditSourceName = "reddit"
	redditBaseURL    = "https://www.reddit.com"
	redditTimeout    = 30 * time.Second
	redditUserAgent  = "noisepan/1.0"
	redditRateLimit  = 1 * time.Second
)

// RedditSource fetches posts from public subreddits via Reddit's JSON API.
type RedditSource struct {
	subreddits []string
	client     *http.Client
	baseURL    string
}

// NewReddit creates a Reddit source. At least one subreddit is required.
func NewReddit(subreddits []string) (*RedditSource, error) {
	if len(subreddits) == 0 {
		return nil, errors.New("reddit: at least one subreddit is required")
	}
	return &RedditSource{
		subreddits: subreddits,
		client:     &http.Client{Timeout: redditTimeout},
		baseURL:    redditBaseURL,
	}, nil
}

func (rs *RedditSource) Name() string {
	return redditSourceName
}

func (rs *RedditSource) Fetch(since time.Time) ([]Post, error) {
	var posts []Post

	for i, sub := range rs.subreddits {
		if i > 0 {
			time.Sleep(redditRateLimit)
		}

		items, err := rs.fetchSubreddit(sub, since)
		if err != nil {
			fmt.Printf("  reddit: r/%s: %v\n", sub, err)
			continue
		}
		posts = append(posts, items...)
	}

	return posts, nil
}

func (rs *RedditSource) fetchSubreddit(subreddit string, since time.Time) ([]Post, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redditTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/r/%s/new.json?limit=100", rs.baseURL, subreddit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", redditUserAgent)

	resp, err := rs.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch r/%s: %w", subreddit, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("r/%s: status %d", subreddit, resp.StatusCode)
	}

	var listing redditListing
	if err := json.NewDecoder(resp.Body).Decode(&listing); err != nil {
		return nil, fmt.Errorf("decode r/%s: %w", subreddit, err)
	}

	return postsFromListing(listing, subreddit, since), nil
}

func postsFromListing(listing redditListing, subreddit string, since time.Time) []Post {
	var posts []Post
	for _, child := range listing.Data.Children {
		p := child.Data
		postedAt := time.Unix(int64(p.CreatedUTC), 0).UTC()
		if postedAt.Before(since) {
			continue
		}

		text := p.Title
		if strings.TrimSpace(p.Selftext) != "" {
			text = p.Title + "\n\n" + p.Selftext
		}

		posts = append(posts, Post{
			Source:     redditSourceName,
			Channel:    subreddit,
			ExternalID: p.ID,
			Text:       text,
			URL:        redditBaseURL + p.Permalink,
			PostedAt:   postedAt,
		})
	}
	return posts
}

type redditListing struct {
	Data struct {
		Children []redditChild `json:"children"`
	} `json:"data"`
}

type redditChild struct {
	Data redditPost `json:"data"`
}

type redditPost struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Selftext   string  `json:"selftext"`
	URL        string  `json:"url"`
	Permalink  string  `json:"permalink"`
	CreatedUTC float64 `json:"created_utc"`
}
