package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	hnSourceName   = "hn"
	hnChannelName  = "Hacker News"
	hnAPIBase      = "https://hacker-news.firebaseio.com/v0"
	hnFetchTimeout = 30 * time.Second
	hnMaxStories   = 200
	hnMaxWorkers   = 5
)

// HNSource fetches top stories from Hacker News via the Firebase API.
type HNSource struct {
	minPoints int
}

// NewHN creates a Hacker News source. minPoints filters stories below the threshold.
func NewHN(minPoints int) (*HNSource, error) {
	if minPoints < 1 {
		return nil, errors.New("hn: min_points must be at least 1")
	}
	return &HNSource{minPoints: minPoints}, nil
}

func (h *HNSource) Name() string {
	return hnSourceName
}

// hnItem represents a Hacker News story from the API.
type hnItem struct {
	ID          int    `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Score       int    `json:"score"`
	Time        int64  `json:"time"`
	Descendants int    `json:"descendants"`
	By          string `json:"by"`
}

// hnAPIBaseURL allows tests to override the API endpoint.
var hnAPIBaseURL = hnAPIBase

func (h *HNSource) Fetch(since time.Time) ([]Post, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hnFetchTimeout)
	defer cancel()

	// Fetch top story IDs.
	ids, err := h.fetchTopStories(ctx)
	if err != nil {
		return nil, fmt.Errorf("hn: fetch top stories: %w", err)
	}

	// Cap at hnMaxStories.
	if len(ids) > hnMaxStories {
		ids = ids[:hnMaxStories]
	}

	type result struct {
		post *Post
		err  error
	}

	jobs := make(chan int, len(ids))
	results := make(chan result, len(ids))

	workers := hnMaxWorkers
	if len(ids) < workers {
		workers = len(ids)
	}

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				item, err := h.fetchItem(ctx, id)
				if err != nil {
					results <- result{err: err}
					continue
				}
				if item.Type != "story" || item.Score < h.minPoints {
					results <- result{}
					continue
				}
				postedAt := time.Unix(item.Time, 0)
				if postedAt.Before(since) {
					results <- result{}
					continue
				}
				results <- result{post: &Post{
					Source:     hnSourceName,
					Channel:    hnChannelName,
					ExternalID: strconv.Itoa(item.ID),
					Text:       item.Title,
					URL:        item.URL,
					PostedAt:   postedAt,
				}}
			}
		}()
	}

	for _, id := range ids {
		jobs <- id
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	var posts []Post
	for r := range results {
		if r.err != nil {
			fmt.Printf("  hn: %v\n", r.err)
			continue
		}
		if r.post != nil {
			posts = append(posts, *r.post)
		}
	}

	return posts, nil
}

func (h *HNSource) fetchTopStories(ctx context.Context) ([]int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hnAPIBaseURL+"/topstories.json", nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("topstories: HTTP %d", resp.StatusCode)
	}

	var ids []int
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, fmt.Errorf("topstories: %w", err)
	}
	return ids, nil
}

func (h *HNSource) fetchItem(ctx context.Context, id int) (*hnItem, error) {
	url := fmt.Sprintf("%s/item/%d.json", hnAPIBaseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("item %d: %w", id, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("item %d: HTTP %d", id, resp.StatusCode)
	}

	var item hnItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("item %d: %w", id, err)
	}
	return &item, nil
}
