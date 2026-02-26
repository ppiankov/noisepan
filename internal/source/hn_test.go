package source

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewHN(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		h, err := NewHN(100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if h == nil {
			t.Fatal("expected non-nil source")
		}
	})

	t.Run("zero points", func(t *testing.T) {
		_, err := NewHN(0)
		if err == nil {
			t.Fatal("expected error for zero min_points")
		}
	})

	t.Run("negative points", func(t *testing.T) {
		_, err := NewHN(-1)
		if err == nil {
			t.Fatal("expected error for negative min_points")
		}
	})
}

func TestHNSource_Name(t *testing.T) {
	h, _ := NewHN(100)
	if h.Name() != "hn" {
		t.Errorf("name = %q, want hn", h.Name())
	}
}

func TestHNFetch(t *testing.T) {
	now := time.Now()
	recentUnix := now.Add(-1 * time.Hour).Unix()
	oldUnix := now.Add(-48 * time.Hour).Unix()

	items := map[string]hnItem{
		"1": {ID: 1, Type: "story", Title: "Denmark ditching Microsoft", URL: "https://example.com/1", Score: 769, Time: recentUnix},
		"2": {ID: 2, Type: "story", Title: "Low score post", URL: "https://example.com/2", Score: 5, Time: recentUnix},
		"3": {ID: 3, Type: "story", Title: "Old post", URL: "https://example.com/3", Score: 500, Time: oldUnix},
		"4": {ID: 4, Type: "job", Title: "Hiring at BigCo", URL: "https://example.com/4", Score: 200, Time: recentUnix},
		"5": {ID: 5, Type: "story", Title: "Anthropic safety pledge dropped", URL: "https://example.com/5", Score: 620, Time: recentUnix},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/topstories.json" {
			_ = json.NewEncoder(w).Encode([]int{1, 2, 3, 4, 5})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/item/") {
			idStr := strings.TrimPrefix(r.URL.Path, "/item/")
			idStr = strings.TrimSuffix(idStr, ".json")
			item, ok := items[idStr]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(item)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	oldBase := hnAPIBaseURL
	hnAPIBaseURL = ts.URL
	t.Cleanup(func() { hnAPIBaseURL = oldBase })

	h, err := NewHN(100)
	if err != nil {
		t.Fatalf("NewHN: %v", err)
	}

	posts, err := h.Fetch(now.Add(-24 * time.Hour))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	// Should get: #1 (high score, recent, story) and #5 (high score, recent, story)
	// Filtered out: #2 (score < 100), #3 (old), #4 (type = job)
	if len(posts) != 2 {
		t.Fatalf("got %d posts, want 2", len(posts))
	}

	// Verify post fields (order may vary due to parallel fetching).
	titles := make(map[string]bool)
	for _, p := range posts {
		titles[p.Text] = true
		if p.Source != "hn" {
			t.Errorf("source = %q, want hn", p.Source)
		}
		if p.Channel != "Hacker News" {
			t.Errorf("channel = %q, want Hacker News", p.Channel)
		}
		if p.URL == "" {
			t.Error("url is empty")
		}
	}

	if !titles["Denmark ditching Microsoft"] {
		t.Error("missing expected post: Denmark ditching Microsoft")
	}
	if !titles["Anthropic safety pledge dropped"] {
		t.Error("missing expected post: Anthropic safety pledge dropped")
	}
}

func TestHNFetch_Empty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/topstories.json" {
			fmt.Fprint(w, "[]")
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	oldBase := hnAPIBaseURL
	hnAPIBaseURL = ts.URL
	t.Cleanup(func() { hnAPIBaseURL = oldBase })

	h, _ := NewHN(100)
	posts, err := h.Fetch(time.Now().Add(-24 * time.Hour))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(posts) != 0 {
		t.Errorf("got %d posts, want 0", len(posts))
	}
}
