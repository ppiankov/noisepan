package source

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func makeListing(posts ...redditPost) redditListing {
	var children []redditChild
	for _, p := range posts {
		children = append(children, redditChild{Data: p})
	}
	return redditListing{Data: struct {
		Children []redditChild `json:"children"`
	}{Children: children}}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func redditWithTransport(subreddits []string, rt roundTripFunc) *RedditSource {
	rs, _ := NewReddit(subreddits)
	rs.baseURL = "https://reddit.test"
	rs.client = &http.Client{
		Timeout:   redditTimeout,
		Transport: rt,
	}
	return rs
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return string(b)
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestNewReddit_EmptySubreddits(t *testing.T) {
	_, err := NewReddit(nil)
	if err == nil {
		t.Fatal("expected error for nil subreddits")
	}

	_, err = NewReddit([]string{})
	if err == nil {
		t.Fatal("expected error for empty subreddits")
	}
}

func TestNewReddit_Valid(t *testing.T) {
	rs, err := NewReddit([]string{"devops"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rs == nil {
		t.Fatal("expected non-nil source")
	}
}

func TestRedditSource_Name(t *testing.T) {
	rs, _ := NewReddit([]string{"devops"})
	if rs.Name() != "reddit" {
		t.Errorf("name = %q, want reddit", rs.Name())
	}
}

func TestReddit_SuccessfulFetch(t *testing.T) {
	now := time.Now()
	rs := redditWithTransport([]string{"devops"}, func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("User-Agent") != redditUserAgent {
			t.Errorf("user-agent = %q, want %q", r.Header.Get("User-Agent"), redditUserAgent)
		}
		if r.URL.Path != "/r/devops/new.json" {
			t.Errorf("path = %q, want /r/devops/new.json", r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "100" {
			t.Errorf("limit query = %q, want 100", got)
		}

		listing := makeListing(
			redditPost{
				ID:         "abc123",
				Title:      "CVE Alert",
				Selftext:   "Critical vulnerability found",
				Permalink:  "/r/devops/comments/abc123/cve_alert/",
				CreatedUTC: float64(now.Unix()),
			},
			redditPost{
				ID:         "def456",
				Title:      "Link Post",
				Selftext:   "",
				URL:        "https://example.com",
				Permalink:  "/r/devops/comments/def456/link_post/",
				CreatedUTC: float64(now.Unix()),
			},
		)
		return response(http.StatusOK, mustJSON(t, listing)), nil
	})

	posts, err := rs.Fetch(now.Add(-1 * time.Hour))
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	if len(posts) != 2 {
		t.Fatalf("got %d posts, want 2", len(posts))
	}

	p := posts[0]
	if p.Source != "reddit" {
		t.Errorf("source = %q", p.Source)
	}
	if p.Channel != "devops" {
		t.Errorf("channel = %q", p.Channel)
	}
	if p.ExternalID != "abc123" {
		t.Errorf("external_id = %q", p.ExternalID)
	}
	if !strings.Contains(p.Text, "CVE Alert") || !strings.Contains(p.Text, "Critical vulnerability") {
		t.Errorf("text = %q, want title + selftext", p.Text)
	}
	if !strings.Contains(p.URL, "/r/devops/comments/abc123") {
		t.Errorf("url = %q", p.URL)
	}

	// Link post: no selftext, text should be title only
	if posts[1].Text != "Link Post" {
		t.Errorf("link post text = %q, want just title", posts[1].Text)
	}
}

func TestReddit_SinceFilter(t *testing.T) {
	now := time.Now()
	old := now.Add(-48 * time.Hour)
	since := now.Add(-24 * time.Hour)

	rs := redditWithTransport([]string{"test"}, func(_ *http.Request) (*http.Response, error) {
		listing := makeListing(
			redditPost{ID: "new1", Title: "New", CreatedUTC: float64(now.Unix()), Permalink: "/r/test/new1"},
			redditPost{ID: "old1", Title: "Old", CreatedUTC: float64(old.Unix()), Permalink: "/r/test/old1"},
			redditPost{ID: "new2", Title: "Also New", CreatedUTC: float64(now.Add(-1 * time.Hour).Unix()), Permalink: "/r/test/new2"},
		)
		return response(http.StatusOK, mustJSON(t, listing)), nil
	})

	posts, err := rs.Fetch(since)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	if len(posts) != 2 {
		t.Fatalf("got %d posts, want 2 (filtered old)", len(posts))
	}
}

func TestReddit_EmptyListing(t *testing.T) {
	rs := redditWithTransport([]string{"empty"}, func(_ *http.Request) (*http.Response, error) {
		listing := makeListing()
		return response(http.StatusOK, mustJSON(t, listing)), nil
	})

	posts, err := rs.Fetch(time.Now().Add(-24 * time.Hour))
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(posts) != 0 {
		t.Errorf("got %d posts, want 0", len(posts))
	}
}

func TestReddit_APIError(t *testing.T) {
	rs := redditWithTransport([]string{"ratelimited"}, func(_ *http.Request) (*http.Response, error) {
		return response(http.StatusTooManyRequests, ""), nil
	})
	posts, err := rs.Fetch(time.Now().Add(-24 * time.Hour))
	if err != nil {
		t.Fatalf("fetch should not return error (non-fatal): %v", err)
	}
	if len(posts) != 0 {
		t.Errorf("got %d posts, want 0", len(posts))
	}
}

func TestReddit_MalformedJSON(t *testing.T) {
	rs := redditWithTransport([]string{"broken"}, func(_ *http.Request) (*http.Response, error) {
		return response(http.StatusOK, "{{{not json"), nil
	})
	posts, err := rs.Fetch(time.Now().Add(-24 * time.Hour))
	if err != nil {
		t.Fatalf("fetch should not return error (non-fatal): %v", err)
	}
	if len(posts) != 0 {
		t.Errorf("got %d posts, want 0", len(posts))
	}
}

func TestPostsFromListing(t *testing.T) {
	now := time.Now()
	since := now.Add(-24 * time.Hour)

	listing := makeListing(
		redditPost{
			ID:         "abc",
			Title:      "Test Post",
			Selftext:   "Body text",
			Permalink:  "/r/test/comments/abc/test_post/",
			CreatedUTC: float64(now.Unix()),
		},
	)

	posts := postsFromListing(listing, "test", since)
	if len(posts) != 1 {
		t.Fatalf("got %d posts, want 1", len(posts))
	}

	p := posts[0]
	if p.Text != "Test Post\n\nBody text" {
		t.Errorf("text = %q, want title + body", p.Text)
	}
	if p.URL != redditBaseURL+"/r/test/comments/abc/test_post/" {
		t.Errorf("url = %q", p.URL)
	}
}
