package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ppiankov/noisepan/internal/store"
	"github.com/spf13/cobra"
)

func TestRescoreAction(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "noisepan.db")
	scriptPath := filepath.Join(tmpDir, "forge-plan.sh")

	writeTestForgePlanScript(t, scriptPath)
	writeTestConfig(t, tmpDir, dbPath, scriptPath)
	writeTestTaste(t, tmpDir)

	oldConfigDir := configDir
	oldRescoreSince := rescoreSince
	t.Cleanup(func() {
		configDir = oldConfigDir
		rescoreSince = oldRescoreSince
	})

	configDir = tmpDir
	rescoreSince = ""

	// Seed the DB with posts and initial scores
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	base := time.Now().Add(-2 * time.Hour)
	p1, err := st.InsertPost(context.Background(), store.PostInput{
		Source: "rss", Channel: "blog", ExternalID: "1",
		Text:     "CVE-2026-1234 kubernetes breaking change affects control plane",
		PostedAt: base, FetchedAt: base.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	p2, err := st.InsertPost(context.Background(), store.PostInput{
		Source: "rss", Channel: "blog", ExternalID: "2",
		Text:     "join our webinar on best practices",
		PostedAt: base.Add(time.Hour), FetchedAt: base.Add(time.Hour + time.Minute),
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Save stale scores (wrong tiers to verify they get replaced)
	if err := st.SaveScore(context.Background(), store.Score{
		PostID: p1.ID, Score: 0, Tier: "ignore", ScoredAt: base.Add(2 * time.Hour),
	}); err != nil {
		t.Fatalf("save score: %v", err)
	}
	if err := st.SaveScore(context.Background(), store.Score{
		PostID: p2.ID, Score: 99, Tier: "read_now", ScoredAt: base.Add(2 * time.Hour),
	}); err != nil {
		t.Fatalf("save score: %v", err)
	}
	_ = st.Close()

	// Run rescore
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := rescoreAction(cmd, nil); err != nil {
		t.Fatalf("rescore: %v", err)
	}

	output := buf.String()
	if !containsStr(output, "Deleted 2 existing scores") {
		t.Errorf("expected delete message, got:\n%s", output)
	}
	if !containsStr(output, "Rescored 2 posts") {
		t.Errorf("expected rescore message, got:\n%s", output)
	}

	// Verify scores were recalculated correctly
	st2, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer func() { _ = st2.Close() }()

	posts, err := st2.GetPosts(context.Background(), time.Time{}, "")
	if err != nil {
		t.Fatalf("get posts: %v", err)
	}

	tierMap := make(map[string]string)
	for _, pws := range posts {
		if pws.Score == nil {
			t.Fatalf("post %d has nil score after rescore", pws.Post.ID)
		}
		tierMap[pws.Post.ExternalID] = pws.Score.Tier
	}

	// Post 1: "CVE-2026-1234 kubernetes breaking change" → cve(5) + kubernetes(3) + breaking change rule(2) = 10 → read_now
	if tierMap["1"] != "read_now" {
		t.Errorf("post 1 tier = %q, want read_now", tierMap["1"])
	}
	// Post 2: "join our webinar on best practices" → webinar(-4) = -4 → ignore
	if tierMap["2"] != "ignore" {
		t.Errorf("post 2 tier = %q, want ignore", tierMap["2"])
	}
}

func TestRescoreAction_NoPosts(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "noisepan.db")
	scriptPath := filepath.Join(tmpDir, "forge-plan.sh")

	writeTestForgePlanScript(t, scriptPath)
	writeTestConfig(t, tmpDir, dbPath, scriptPath)
	writeTestTaste(t, tmpDir)

	oldConfigDir := configDir
	oldRescoreSince := rescoreSince
	t.Cleanup(func() {
		configDir = oldConfigDir
		rescoreSince = oldRescoreSince
	})

	configDir = tmpDir
	rescoreSince = ""

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := rescoreAction(cmd, nil); err != nil {
		t.Fatalf("rescore: %v", err)
	}

	output := buf.String()
	if !containsStr(output, "Deleted 0 existing scores") {
		t.Errorf("expected 0 deleted, got:\n%s", output)
	}
	if !containsStr(output, "Rescored 0 posts") {
		t.Errorf("expected 0 rescored, got:\n%s", output)
	}
}

func containsStr(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && searchStr(s, substr))
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
