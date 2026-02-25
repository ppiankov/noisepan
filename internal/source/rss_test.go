package source

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

func TestNewRSS_EmptyFeeds(t *testing.T) {
	_, err := NewRSS(nil)
	if err == nil {
		t.Fatal("expected error for nil feeds")
	}

	_, err = NewRSS([]string{})
	if err == nil {
		t.Fatal("expected error for empty feeds")
	}
}

func TestNewRSS_Valid(t *testing.T) {
	rs, err := NewRSS([]string{"https://example.com/feed.xml"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rs == nil {
		t.Fatal("expected non-nil source")
	}
}

func TestRSSSource_Name(t *testing.T) {
	rs, _ := NewRSS([]string{"https://example.com/feed.xml"})
	if rs.Name() != "rss" {
		t.Errorf("name = %q, want rss", rs.Name())
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple tags", "<p>hello</p>", "hello"},
		{"nested tags", "<div><p>hello</p></div>", "hello"},
		{"entities", "&amp; &lt; &gt;", "& < >"},
		{"mixed", "<b>bold</b> &amp; <i>italic</i>", "bold  &  italic"},
		{"empty", "", ""},
		{"no html", "plain text", "plain text"},
		{"self-closing", "line<br/>break", "line break"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHTML(tt.input)
			if got != tt.want {
				t.Errorf("stripHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestItemPublishedTime(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-time.Hour)

	t.Run("published", func(t *testing.T) {
		item := &gofeed.Item{PublishedParsed: &now}
		if got := itemPublishedTime(item); !got.Equal(now) {
			t.Errorf("got %v, want %v", got, now)
		}
	})

	t.Run("updated fallback", func(t *testing.T) {
		item := &gofeed.Item{UpdatedParsed: &earlier}
		if got := itemPublishedTime(item); !got.Equal(earlier) {
			t.Errorf("got %v, want %v", got, earlier)
		}
	})

	t.Run("published preferred", func(t *testing.T) {
		item := &gofeed.Item{PublishedParsed: &now, UpdatedParsed: &earlier}
		if got := itemPublishedTime(item); !got.Equal(now) {
			t.Errorf("got %v (updated), want %v (published)", got, now)
		}
	})

	t.Run("zero", func(t *testing.T) {
		item := &gofeed.Item{}
		if got := itemPublishedTime(item); !got.IsZero() {
			t.Errorf("got %v, want zero", got)
		}
	})
}

func TestItemID(t *testing.T) {
	t.Run("guid", func(t *testing.T) {
		item := &gofeed.Item{GUID: "abc-123", Link: "https://example.com/post"}
		if got := itemID(item); got != "abc-123" {
			t.Errorf("got %q, want abc-123", got)
		}
	})

	t.Run("link fallback", func(t *testing.T) {
		item := &gofeed.Item{Link: "https://example.com/post"}
		if got := itemID(item); got != "https://example.com/post" {
			t.Errorf("got %q, want link", got)
		}
	})

	t.Run("empty", func(t *testing.T) {
		item := &gofeed.Item{}
		if got := itemID(item); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestFeedLabel(t *testing.T) {
	t.Run("title", func(t *testing.T) {
		feed := &gofeed.Feed{Title: "My Blog"}
		if got := feedLabel(feed, "https://example.com/feed.xml"); got != "My Blog" {
			t.Errorf("got %q, want My Blog", got)
		}
	})

	t.Run("url fallback", func(t *testing.T) {
		feed := &gofeed.Feed{}
		if got := feedLabel(feed, "https://example.com/feed.xml"); got != "https://example.com/feed.xml" {
			t.Errorf("got %q, want URL", got)
		}
	})
}

func TestItemText(t *testing.T) {
	t.Run("title and content", func(t *testing.T) {
		item := &gofeed.Item{
			Title:   "Breaking Change",
			Content: "<p>Details about the change</p>",
		}
		got := itemText(item)
		if got != "Breaking Change\n\nDetails about the change" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("description fallback", func(t *testing.T) {
		item := &gofeed.Item{
			Title:       "Alert",
			Description: "Short description",
		}
		got := itemText(item)
		if got != "Alert\n\nShort description" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("title in content", func(t *testing.T) {
		item := &gofeed.Item{
			Title:   "Alert",
			Content: "Alert: something happened",
		}
		got := itemText(item)
		if got != "Alert: something happened" {
			t.Errorf("got %q, expected no title duplication", got)
		}
	})

	t.Run("no title", func(t *testing.T) {
		item := &gofeed.Item{Content: "Just content"}
		got := itemText(item)
		if got != "Just content" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("empty", func(t *testing.T) {
		item := &gofeed.Item{}
		got := itemText(item)
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestPostsFromFeed(t *testing.T) {
	now := time.Now()
	recent := now.Add(-1 * time.Hour)
	old := now.Add(-48 * time.Hour)
	since := now.Add(-24 * time.Hour)

	feed := &gofeed.Feed{
		Title: "DevOps Weekly",
		Items: []*gofeed.Item{
			{
				GUID:            "1",
				Title:           "Recent Post",
				Description:     "Recent content",
				Link:            "https://example.com/1",
				PublishedParsed: &recent,
			},
			{
				GUID:            "2",
				Title:           "Old Post",
				Description:     "Old content",
				Link:            "https://example.com/2",
				PublishedParsed: &old,
			},
			{
				Title:       "No Date",
				Description: "No date content",
			},
		},
	}

	posts := postsFromFeed(feed, "https://example.com/feed.xml", since)

	if len(posts) != 1 {
		t.Fatalf("got %d posts, want 1 (only recent)", len(posts))
	}

	p := posts[0]
	if p.Source != "rss" {
		t.Errorf("source = %q, want rss", p.Source)
	}
	if p.Channel != "DevOps Weekly" {
		t.Errorf("channel = %q, want DevOps Weekly", p.Channel)
	}
	if p.ExternalID != "1" {
		t.Errorf("external_id = %q, want 1", p.ExternalID)
	}
	if p.URL != "https://example.com/1" {
		t.Errorf("url = %q", p.URL)
	}
}

func TestPostsFromFeed_Empty(t *testing.T) {
	feed := &gofeed.Feed{Title: "Empty Feed"}
	posts := postsFromFeed(feed, "https://example.com/feed.xml", time.Now())
	if len(posts) != 0 {
		t.Errorf("got %d posts, want 0", len(posts))
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{fmt.Errorf("timeout exceeded"), true},
		{fmt.Errorf("Timeout waiting for response"), true},
		{fmt.Errorf("connection refused"), true},
		{fmt.Errorf("no such host"), true},
		{fmt.Errorf("HTTP 429 Too Many Requests"), true},
		{fmt.Errorf("HTTP 500 Internal Server Error"), true},
		{fmt.Errorf("HTTP 502 Bad Gateway"), true},
		{fmt.Errorf("HTTP 503 Service Unavailable"), true},
		{fmt.Errorf("HTTP 504 Gateway Timeout"), true},
		{fmt.Errorf("HTTP 404 Not Found"), false},
		{fmt.Errorf("HTTP 403 Forbidden"), false},
		{fmt.Errorf("parse error: invalid XML"), false},
	}

	for _, tt := range tests {
		got := isRetryableError(tt.err)
		if got != tt.want {
			t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

func TestFetchWithRetry_TransientThenSuccess(t *testing.T) {
	// Override sleep to be instant in tests
	oldSleep := rssSleepFunc
	rssSleepFunc = func(_ time.Duration) {}
	t.Cleanup(func() { rssSleepFunc = oldSleep })

	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		now := time.Now().Format(time.RFC3339)
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, `<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <item>
      <title>Test Item</title>
      <link>https://example.com/1</link>
      <guid>1</guid>
      <pubDate>%s</pubDate>
    </item>
  </channel>
</rss>`, now)
	}))
	defer ts.Close()

	posts, err := fetchWithRetry(ts.URL, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("fetchWithRetry: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("got %d posts, want 1", len(posts))
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", calls.Load())
	}
}

func TestFetchWithRetry_PermanentFailure(t *testing.T) {
	oldSleep := rssSleepFunc
	rssSleepFunc = func(_ time.Duration) {}
	t.Cleanup(func() { rssSleepFunc = oldSleep })

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	_, err := fetchWithRetry(ts.URL, time.Now().Add(-time.Hour))
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestFetchWithRetry_AllRetriesFail(t *testing.T) {
	oldSleep := rssSleepFunc
	rssSleepFunc = func(_ time.Duration) {}
	t.Cleanup(func() { rssSleepFunc = oldSleep })

	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	_, err := fetchWithRetry(ts.URL, time.Now().Add(-time.Hour))
	if err == nil {
		t.Fatal("expected error after all retries exhausted")
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", calls.Load())
	}
}

func TestFeedDomain(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://reddit.com/r/devops/.rss", "reddit.com"},
		{"https://www.example.com/feed.xml", "www.example.com"},
		{"http://localhost:8080/feed", "localhost:8080"},
		{"not-a-url", "not-a-url"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := feedDomain(tt.url); got != tt.want {
				t.Errorf("feedDomain(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestFetch_DomainSerialization(t *testing.T) {
	oldSleep := rssSleepFunc
	rssSleepFunc = func(_ time.Duration) {}
	t.Cleanup(func() { rssSleepFunc = oldSleep })

	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	now := time.Now().Format(time.RFC3339)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cur := concurrent.Add(1)
		for {
			old := maxConcurrent.Load()
			if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		concurrent.Add(-1)

		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, `<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Test</title>
    <item>
      <title>Post</title>
      <link>https://example.com/1</link>
      <guid>1</guid>
      <pubDate>%s</pubDate>
    </item>
  </channel>
</rss>`, now)
	}))
	defer ts.Close()

	// 5 feeds on the same domain — must be serialized.
	feeds := make([]string, 5)
	for i := range feeds {
		feeds[i] = fmt.Sprintf("%s/feed/%d", ts.URL, i)
	}

	rs, err := NewRSS(feeds)
	if err != nil {
		t.Fatalf("NewRSS: %v", err)
	}

	posts, err := rs.Fetch(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(posts) != 5 {
		t.Errorf("got %d posts, want 5", len(posts))
	}
	if maxConcurrent.Load() > 1 {
		t.Errorf("max concurrent requests to same domain = %d, want 1", maxConcurrent.Load())
	}
}

func TestFetch_DomainDelay(t *testing.T) {
	oldSleep := rssSleepFunc
	var mu sync.Mutex
	var delays []time.Duration
	rssSleepFunc = func(d time.Duration) {
		mu.Lock()
		delays = append(delays, d)
		mu.Unlock()
	}
	t.Cleanup(func() { rssSleepFunc = oldSleep })

	now := time.Now().Format(time.RFC3339)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, `<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Test</title>
    <item>
      <title>Post</title>
      <link>https://example.com/1</link>
      <guid>1</guid>
      <pubDate>%s</pubDate>
    </item>
  </channel>
</rss>`, now)
	}))
	defer ts.Close()

	// 3 feeds on the same domain — expect 2 domain delays.
	feeds := []string{
		ts.URL + "/feed/a",
		ts.URL + "/feed/b",
		ts.URL + "/feed/c",
	}

	rs, err := NewRSS(feeds)
	if err != nil {
		t.Fatalf("NewRSS: %v", err)
	}

	_, err = rs.Fetch(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	domainDelays := 0
	for _, d := range delays {
		if d == rssDomainDelay {
			domainDelays++
		}
	}
	if domainDelays != 2 {
		t.Errorf("domain delays = %d, want 2 (between 3 same-domain feeds)", domainDelays)
	}
}
