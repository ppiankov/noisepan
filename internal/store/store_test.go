package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "noisepan.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
	})
	return st, path
}

func TestOpenAndMigrate(t *testing.T) {
	st, path := openTestStore(t)

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("db file not created: %v", err)
	}

	var version string
	if err := st.db.QueryRow("SELECT value FROM metadata WHERE key = 'schema_version'").Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != "1" {
		t.Fatalf("unexpected schema version: %s", version)
	}
}

func TestInsertPostUpsertAndHash(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()

	postedAt := time.Date(2026, 2, 16, 10, 0, 0, 0, time.UTC)
	fetchedAt := postedAt.Add(2 * time.Minute)

	post, err := st.InsertPost(ctx, PostInput{
		Source:     "telegram",
		Channel:    "devops",
		ExternalID: "1",
		Text:       "hello world",
		PostedAt:   postedAt,
		FetchedAt:  fetchedAt,
	})
	if err != nil {
		t.Fatalf("insert post: %v", err)
	}

	if post.Snippet == "" {
		t.Fatalf("expected snippet to be populated")
	}

	expectedHash := textHash("hello world", post.Snippet)
	if post.TextHash != expectedHash {
		t.Fatalf("unexpected text hash: %s", post.TextHash)
	}

	_, err = st.InsertPost(ctx, PostInput{
		Source:     "telegram",
		Channel:    "devops",
		ExternalID: "1",
		Text:       "updated text",
		PostedAt:   postedAt,
		FetchedAt:  fetchedAt.Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("upsert post: %v", err)
	}

	var count int
	if err := st.db.QueryRow("SELECT COUNT(*) FROM posts").Scan(&count); err != nil {
		t.Fatalf("count posts: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 post, got %d", count)
	}

	var text string
	var hash string
	if err := st.db.QueryRow("SELECT text, text_hash FROM posts WHERE source = ? AND channel = ? AND external_id = ?", "telegram", "devops", "1").Scan(&text, &hash); err != nil {
		t.Fatalf("fetch updated post: %v", err)
	}
	if text != "updated text" {
		t.Fatalf("expected updated text, got %q", text)
	}
	if hash != textHash("updated text", "") {
		t.Fatalf("expected updated hash, got %q", hash)
	}
}

func TestGetUnscoredAndSaveScore(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()

	postedAt := time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC)
	fetchedAt := postedAt.Add(1 * time.Minute)

	post, err := st.InsertPost(ctx, PostInput{
		Source:     "rss",
		Channel:    "devops",
		ExternalID: "abc",
		Text:       "rolling update",
		PostedAt:   postedAt,
		FetchedAt:  fetchedAt,
	})
	if err != nil {
		t.Fatalf("insert post: %v", err)
	}

	unscored, err := st.GetUnscored(ctx)
	if err != nil {
		t.Fatalf("get unscored: %v", err)
	}
	if len(unscored) != 1 {
		t.Fatalf("expected 1 unscored, got %d", len(unscored))
	}

	explanation := json.RawMessage(`{"why":"signal"}`)
	err = st.SaveScore(ctx, Score{
		PostID:      post.ID,
		Score:       42,
		Labels:      []string{"release", "k8s"},
		Tier:        "read_now",
		ScoredAt:    postedAt.Add(30 * time.Minute),
		Explanation: explanation,
	})
	if err != nil {
		t.Fatalf("save score: %v", err)
	}

	unscored, err = st.GetUnscored(ctx)
	if err != nil {
		t.Fatalf("get unscored after score: %v", err)
	}
	if len(unscored) != 0 {
		t.Fatalf("expected 0 unscored, got %d", len(unscored))
	}

	var (
		scoreVal  int
		labelsVal string
		tierVal   string
		explVal   sql.NullString
	)
	if err := st.db.QueryRow("SELECT score, labels, tier, explanation FROM scores WHERE post_id = ?", post.ID).Scan(&scoreVal, &labelsVal, &tierVal, &explVal); err != nil {
		t.Fatalf("fetch score: %v", err)
	}
	if scoreVal != 42 {
		t.Fatalf("expected score 42, got %d", scoreVal)
	}
	if tierVal != "read_now" {
		t.Fatalf("expected tier read_now, got %s", tierVal)
	}

	var labels []string
	if err := json.Unmarshal([]byte(labelsVal), &labels); err != nil {
		t.Fatalf("decode labels: %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(labels))
	}
	if !explVal.Valid || explVal.String == "" {
		t.Fatalf("expected explanation to be stored")
	}
}

func TestGetPosts(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()

	base := time.Date(2026, 2, 16, 8, 0, 0, 0, time.UTC)

	oldPost, err := st.InsertPost(ctx, PostInput{
		Source:     "reddit",
		Channel:    "devops",
		ExternalID: "old",
		Text:       "old post",
		PostedAt:   base,
		FetchedAt:  base.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("insert old post: %v", err)
	}

	newPost, err := st.InsertPost(ctx, PostInput{
		Source:     "reddit",
		Channel:    "devops",
		ExternalID: "new",
		Text:       "new post",
		PostedAt:   base.Add(2 * time.Hour),
		FetchedAt:  base.Add(2*time.Hour + 2*time.Minute),
	})
	if err != nil {
		t.Fatalf("insert new post: %v", err)
	}

	if err := st.SaveScore(ctx, Score{
		PostID:   newPost.ID,
		Score:    10,
		Labels:   []string{"signal"},
		Tier:     "read_now",
		ScoredAt: base.Add(3 * time.Hour),
	}); err != nil {
		t.Fatalf("save score: %v", err)
	}

	posts, err := st.GetPosts(ctx, base.Add(-time.Minute), "")
	if err != nil {
		t.Fatalf("get posts: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(posts))
	}
	if posts[0].Post.ID != newPost.ID {
		t.Fatalf("expected newest post first")
	}
	if posts[0].Score == nil || posts[0].Score.Tier != "read_now" {
		t.Fatalf("expected score for newest post")
	}
	if posts[1].Post.ID != oldPost.ID {
		t.Fatalf("expected older post second")
	}
	if posts[1].Score != nil {
		t.Fatalf("expected no score for older post")
	}

	filtered, err := st.GetPosts(ctx, base.Add(-time.Minute), "read_now")
	if err != nil {
		t.Fatalf("get filtered posts: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered post, got %d", len(filtered))
	}
	if filtered[0].Post.ID != newPost.ID {
		t.Fatalf("expected filtered post to be newest")
	}
}

func TestGetPosts_FilterBySource(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()

	base := time.Date(2026, 2, 16, 8, 0, 0, 0, time.UTC)

	_, err := st.InsertPost(ctx, PostInput{
		Source: "rss", Channel: "blog", ExternalID: "1",
		Text: "rss post", PostedAt: base, FetchedAt: base.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	_, err = st.InsertPost(ctx, PostInput{
		Source: "reddit", Channel: "devops", ExternalID: "2",
		Text: "reddit post", PostedAt: base.Add(time.Hour), FetchedAt: base.Add(time.Hour + time.Minute),
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	posts, err := st.GetPosts(ctx, base.Add(-time.Minute), "", PostFilter{Source: "rss"})
	if err != nil {
		t.Fatalf("get posts: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("got %d posts, want 1", len(posts))
	}
	if posts[0].Post.Source != "rss" {
		t.Errorf("source = %q, want rss", posts[0].Post.Source)
	}
}

func TestGetPosts_FilterByChannel(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()

	base := time.Date(2026, 2, 16, 8, 0, 0, 0, time.UTC)

	_, err := st.InsertPost(ctx, PostInput{
		Source: "rss", Channel: "blog", ExternalID: "1",
		Text: "blog post", PostedAt: base, FetchedAt: base.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	_, err = st.InsertPost(ctx, PostInput{
		Source: "rss", Channel: "news", ExternalID: "2",
		Text: "news post", PostedAt: base.Add(time.Hour), FetchedAt: base.Add(time.Hour + time.Minute),
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	posts, err := st.GetPosts(ctx, base.Add(-time.Minute), "", PostFilter{Channel: "blog"})
	if err != nil {
		t.Fatalf("get posts: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("got %d posts, want 1", len(posts))
	}
	if posts[0].Post.Channel != "blog" {
		t.Errorf("channel = %q, want blog", posts[0].Post.Channel)
	}
}

func TestGetPosts_FilterCombined(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()

	base := time.Date(2026, 2, 16, 8, 0, 0, 0, time.UTC)

	for i, p := range []PostInput{
		{Source: "rss", Channel: "blog", ExternalID: "1", Text: "rss blog", PostedAt: base, FetchedAt: base.Add(time.Minute)},
		{Source: "rss", Channel: "news", ExternalID: "2", Text: "rss news", PostedAt: base.Add(time.Hour), FetchedAt: base.Add(time.Hour + time.Minute)},
		{Source: "reddit", Channel: "blog", ExternalID: "3", Text: "reddit blog", PostedAt: base.Add(2 * time.Hour), FetchedAt: base.Add(2*time.Hour + time.Minute)},
	} {
		if _, err := st.InsertPost(ctx, p); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	posts, err := st.GetPosts(ctx, base.Add(-time.Minute), "", PostFilter{Source: "rss", Channel: "blog"})
	if err != nil {
		t.Fatalf("get posts: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("got %d posts, want 1", len(posts))
	}
	if posts[0].Post.Source != "rss" || posts[0].Post.Channel != "blog" {
		t.Errorf("got source=%q channel=%q, want rss/blog", posts[0].Post.Source, posts[0].Post.Channel)
	}
}

func TestDeduplicate(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()

	base := time.Date(2026, 2, 16, 14, 0, 0, 0, time.UTC)

	_, err := st.InsertPost(ctx, PostInput{
		Source:     "telegram",
		Channel:    "chan1",
		ExternalID: "1",
		Text:       "same content",
		PostedAt:   base,
		FetchedAt:  base.Add(1 * time.Minute),
	})
	if err != nil {
		t.Fatalf("insert first duplicate: %v", err)
	}

	_, err = st.InsertPost(ctx, PostInput{
		Source:     "rss",
		Channel:    "chan2",
		ExternalID: "2",
		Text:       "same content",
		PostedAt:   base.Add(2 * time.Hour),
		FetchedAt:  base.Add(2*time.Hour + 1*time.Minute),
	})
	if err != nil {
		t.Fatalf("insert second duplicate: %v", err)
	}

	deleted, err := st.Deduplicate(ctx)
	if err != nil {
		t.Fatalf("deduplicate: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted, got %d", deleted)
	}

	var count int
	if err := st.db.QueryRow("SELECT COUNT(*) FROM posts").Scan(&count); err != nil {
		t.Fatalf("count posts after dedup: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 post after dedup, got %d", count)
	}

	// Verify also_in was recorded
	var alsoCount int
	if err := st.db.QueryRow("SELECT COUNT(*) FROM post_also_in").Scan(&alsoCount); err != nil {
		t.Fatalf("count also_in: %v", err)
	}
	if alsoCount != 1 {
		t.Fatalf("expected 1 also_in, got %d", alsoCount)
	}
}

func TestDeduplicate_AlsoIn(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()

	base := time.Date(2026, 2, 16, 14, 0, 0, 0, time.UTC)

	keeper, err := st.InsertPost(ctx, PostInput{
		Source:     "telegram",
		Channel:    "chan1",
		ExternalID: "1",
		Text:       "duplicate text",
		PostedAt:   base,
		FetchedAt:  base.Add(1 * time.Minute),
	})
	if err != nil {
		t.Fatalf("insert keeper: %v", err)
	}

	_, err = st.InsertPost(ctx, PostInput{
		Source:     "rss",
		Channel:    "feed1",
		ExternalID: "a",
		Text:       "duplicate text",
		PostedAt:   base.Add(1 * time.Hour),
		FetchedAt:  base.Add(1*time.Hour + 1*time.Minute),
	})
	if err != nil {
		t.Fatalf("insert dup: %v", err)
	}

	deleted, err := st.Deduplicate(ctx)
	if err != nil {
		t.Fatalf("deduplicate: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted, got %d", deleted)
	}

	alsoIn, err := st.GetAlsoIn(ctx, []int64{keeper.ID})
	if err != nil {
		t.Fatalf("get also_in: %v", err)
	}
	if len(alsoIn[keeper.ID]) != 1 {
		t.Fatalf("expected 1 also_in entry, got %d", len(alsoIn[keeper.ID]))
	}
	if alsoIn[keeper.ID][0] != "rss/feed1" {
		t.Errorf("also_in = %q, want rss/feed1", alsoIn[keeper.ID][0])
	}
}

func TestDeduplicate_MultipleAlsoIn(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()

	base := time.Date(2026, 2, 16, 14, 0, 0, 0, time.UTC)

	keeper, err := st.InsertPost(ctx, PostInput{
		Source:     "telegram",
		Channel:    "chan1",
		ExternalID: "1",
		Text:       "triple post",
		PostedAt:   base,
		FetchedAt:  base.Add(1 * time.Minute),
	})
	if err != nil {
		t.Fatalf("insert keeper: %v", err)
	}

	_, err = st.InsertPost(ctx, PostInput{
		Source:     "rss",
		Channel:    "feed1",
		ExternalID: "a",
		Text:       "triple post",
		PostedAt:   base.Add(1 * time.Hour),
		FetchedAt:  base.Add(1*time.Hour + 1*time.Minute),
	})
	if err != nil {
		t.Fatalf("insert dup 1: %v", err)
	}

	_, err = st.InsertPost(ctx, PostInput{
		Source:     "reddit",
		Channel:    "sub1",
		ExternalID: "x",
		Text:       "triple post",
		PostedAt:   base.Add(2 * time.Hour),
		FetchedAt:  base.Add(2*time.Hour + 1*time.Minute),
	})
	if err != nil {
		t.Fatalf("insert dup 2: %v", err)
	}

	deleted, err := st.Deduplicate(ctx)
	if err != nil {
		t.Fatalf("deduplicate: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 deleted, got %d", deleted)
	}

	alsoIn, err := st.GetAlsoIn(ctx, []int64{keeper.ID})
	if err != nil {
		t.Fatalf("get also_in: %v", err)
	}
	if len(alsoIn[keeper.ID]) != 2 {
		t.Fatalf("expected 2 also_in entries, got %d", len(alsoIn[keeper.ID]))
	}
}

func TestPruneOld(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	old := now.AddDate(0, 0, -60) // 60 days ago
	recent := now.Add(-1 * time.Hour)

	// Insert old post with score
	oldPost, err := st.InsertPost(ctx, PostInput{
		Source:     "rss",
		Channel:    "blog",
		ExternalID: "old1",
		Text:       "old post",
		PostedAt:   old,
		FetchedAt:  old.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("insert old post: %v", err)
	}
	if err := st.SaveScore(ctx, Score{
		PostID:   oldPost.ID,
		Score:    5,
		Tier:     "skim",
		ScoredAt: old.Add(time.Hour),
	}); err != nil {
		t.Fatalf("save old score: %v", err)
	}

	// Insert recent post
	_, err = st.InsertPost(ctx, PostInput{
		Source:     "rss",
		Channel:    "blog",
		ExternalID: "new1",
		Text:       "new post",
		PostedAt:   recent,
		FetchedAt:  recent.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("insert recent post: %v", err)
	}

	pruned, err := st.PruneOld(ctx, 30)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 1 {
		t.Errorf("pruned = %d, want 1", pruned)
	}

	// Verify only recent post remains
	var count int
	if err := st.db.QueryRow("SELECT COUNT(*) FROM posts").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("posts remaining = %d, want 1", count)
	}

	// Verify old score was deleted
	var scoreCount int
	if err := st.db.QueryRow("SELECT COUNT(*) FROM scores").Scan(&scoreCount); err != nil {
		t.Fatalf("count scores: %v", err)
	}
	if scoreCount != 0 {
		t.Errorf("scores remaining = %d, want 0", scoreCount)
	}
}

func TestPruneOld_ZeroDays(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()

	pruned, err := st.PruneOld(ctx, 0)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 0 {
		t.Errorf("pruned = %d, want 0", pruned)
	}
}

func TestGetAlsoIn_Empty(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()

	alsoIn, err := st.GetAlsoIn(ctx, []int64{999})
	if err != nil {
		t.Fatalf("get also_in: %v", err)
	}
	if len(alsoIn) != 0 {
		t.Errorf("expected empty map, got %v", alsoIn)
	}
}

func TestGetAlsoIn_NilIDs(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()

	alsoIn, err := st.GetAlsoIn(ctx, nil)
	if err != nil {
		t.Fatalf("get also_in: %v", err)
	}
	if alsoIn != nil {
		t.Errorf("expected nil, got %v", alsoIn)
	}
}
