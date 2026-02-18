package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ppiankov/noisepan/internal/store"
	"github.com/spf13/cobra"
)

func TestPipelinePullScoreDigest(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "noisepan.db")
	scriptPath := filepath.Join(tmpDir, "forge-plan.sh")

	writeTestForgePlanScript(t, scriptPath)
	writeTestConfig(t, tmpDir, dbPath, scriptPath)
	writeTestTaste(t, tmpDir)

	oldConfigDir := configDir
	oldDigestSince := digestSince
	oldDigestFormat := digestFormat
	oldDigestSource := digestSource
	oldDigestChannel := digestChannel
	oldNoColor := noColor
	t.Cleanup(func() {
		configDir = oldConfigDir
		digestSince = oldDigestSince
		digestFormat = oldDigestFormat
		digestSource = oldDigestSource
		digestChannel = oldDigestChannel
		noColor = oldNoColor
	})

	configDir = tmpDir
	digestSince = ""
	digestSource = ""
	digestChannel = ""
	noColor = true

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	pullOutput, err := captureStdout(t, func() error {
		return pullAction(cmd, nil)
	})
	if err != nil {
		t.Fatalf("pull action: %v", err)
	}
	requireContains(t, pullOutput, "Pulled 3 posts from 1 channels")

	st := openStoreForPipelineTest(t, dbPath)
	unscored, err := st.GetUnscored(context.Background())
	if err != nil {
		t.Fatalf("get unscored after pull: %v", err)
	}
	if len(unscored) != 3 {
		t.Fatalf("unscored after pull = %d, want 3", len(unscored))
	}
	_ = st.Close()

	digestFormat = "terminal"
	terminalOutput, err := captureStdout(t, func() error {
		return digestAction(cmd, nil)
	})
	if err != nil {
		t.Fatalf("digest terminal: %v", err)
	}
	requireContains(t, terminalOutput, "noisepan — 1 channels, 3 posts, since 7d")
	requireContains(t, terminalOutput, "--- Read Now (1) ---")
	requireContains(t, terminalOutput, "--- Skim (1) ---")
	requireContains(t, terminalOutput, "[10] [ops] forge-plan")
	requireContains(t, terminalOutput, "[3] forge-plan")
	requireContains(t, terminalOutput, "Ignored: 1 posts (noise suppressed)")

	st = openStoreForPipelineTest(t, dbPath)
	posts, err := st.GetPosts(context.Background(), time.Time{}, "")
	if err != nil {
		t.Fatalf("get posts after scoring: %v", err)
	}
	if len(posts) != 3 {
		t.Fatalf("posts after scoring = %d, want 3", len(posts))
	}
	tierCounts := map[string]int{}
	for _, p := range posts {
		if p.Score == nil {
			t.Fatalf("post %d has nil score after digest", p.Post.ID)
		}
		tierCounts[p.Score.Tier]++
	}
	if tierCounts["read_now"] != 1 || tierCounts["skim"] != 1 || tierCounts["ignore"] != 1 {
		t.Fatalf("unexpected tiers: %+v", tierCounts)
	}
	_ = st.Close()

	digestFormat = "json"
	jsonOutput, err := captureStdout(t, func() error {
		return digestAction(cmd, nil)
	})
	if err != nil {
		t.Fatalf("digest json: %v", err)
	}

	var got struct {
		Meta struct {
			Channels   int    `json:"channels"`
			TotalPosts int    `json:"total_posts"`
			Since      string `json:"since"`
		} `json:"meta"`
		ReadNow []struct {
			Source   string   `json:"source"`
			Channel  string   `json:"channel"`
			Score    int      `json:"score"`
			Tier     string   `json:"tier"`
			Labels   []string `json:"labels"`
			Headline string   `json:"headline"`
		} `json:"read_now"`
		Skims []struct {
			Source   string `json:"source"`
			Channel  string `json:"channel"`
			Score    int    `json:"score"`
			Tier     string `json:"tier"`
			Headline string `json:"headline"`
		} `json:"skims"`
		Ignored int `json:"ignored"`
	}
	if err := json.Unmarshal([]byte(jsonOutput), &got); err != nil {
		t.Fatalf("parse json output: %v\noutput:\n%s", err, jsonOutput)
	}

	if got.Meta.Channels != 1 || got.Meta.TotalPosts != 3 || got.Meta.Since != "7d" {
		t.Fatalf("unexpected json meta: %+v", got.Meta)
	}
	if len(got.ReadNow) != 1 || len(got.Skims) != 1 || got.Ignored != 1 {
		t.Fatalf("unexpected json groups: read_now=%d skims=%d ignored=%d", len(got.ReadNow), len(got.Skims), got.Ignored)
	}
	if got.ReadNow[0].Source != "forgeplan" || got.ReadNow[0].Channel != "forge-plan" || got.ReadNow[0].Tier != "read_now" {
		t.Fatalf("unexpected read_now item: %+v", got.ReadNow[0])
	}
	if got.ReadNow[0].Score != 10 {
		t.Fatalf("read_now score = %d, want 10", got.ReadNow[0].Score)
	}
	if got.Skims[0].Score != 3 || got.Skims[0].Tier != "skim" {
		t.Fatalf("unexpected skim item: %+v", got.Skims[0])
	}

	digestFormat = "markdown"
	markdownOutput, err := captureStdout(t, func() error {
		return digestAction(cmd, nil)
	})
	if err != nil {
		t.Fatalf("digest markdown: %v", err)
	}
	requireContains(t, markdownOutput, "# noisepan digest")
	requireContains(t, markdownOutput, "1 channels, 3 posts, since 7d")
	requireContains(t, markdownOutput, "## Read Now (1)")
	requireContains(t, markdownOutput, "### [10] forge-plan")
	requireContains(t, markdownOutput, "## Skim (1)")
	requireContains(t, markdownOutput, "- **[3]** forge-plan")
	requireContains(t, markdownOutput, "*Ignored: 1 posts*")
}

func writeTestForgePlanScript(t *testing.T, path string) {
	t.Helper()

	content := `#!/bin/sh
cat <<'EOF'
Suggested actions

  1. CVE-2026-1111 Kubernetes breaking change affects control plane.
  kubectl apply -f fix.yaml

  2. Kubernetes migration checklist for v1.2.3.
  kubectl rollout status deploy/app

  3. Join our webinar on cluster best practices.
  https://example.com/webinar
EOF
`
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write test forge-plan script: %v", err)
	}
}

func writeTestConfig(t *testing.T, dir, dbPath, scriptPath string) {
	t.Helper()

	content := "sources:\n" +
		"  forgeplan:\n" +
		"    script: \"" + scriptPath + "\"\n" +
		"storage:\n" +
		"  path: \"" + dbPath + "\"\n" +
		"digest:\n" +
		"  timezone: \"UTC\"\n" +
		"  top_n: 10\n" +
		"  include_skims: 10\n" +
		"  since: 168h\n" +
		"summarize:\n" +
		"  mode: heuristic\n" +
		"privacy:\n" +
		"  store_full_text: true\n" +
		"  redact:\n" +
		"    enabled: false\n"

	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test config: %v", err)
	}
}

func writeTestTaste(t *testing.T, dir string) {
	t.Helper()

	content := `weights:
  high_signal:
    "cve": 5
    "kubernetes": 3
  low_signal:
    "webinar": -4
labels: {}
rules:
  - if:
      contains_any: ["breaking change"]
    then:
      score_add: 2
      labels: ["ops"]
thresholds:
  read_now: 7
  skim: 3
  ignore: 0
`
	path := filepath.Join(dir, "taste.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test taste profile: %v", err)
	}
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("open stdout pipe: %v", err)
	}

	os.Stdout = writer
	runErr := fn()
	_ = writer.Close()
	os.Stdout = oldStdout

	out, readErr := io.ReadAll(reader)
	_ = reader.Close()
	if readErr != nil {
		t.Fatalf("read stdout pipe: %v", readErr)
	}
	return string(out), runErr
}

func openStoreForPipelineTest(t *testing.T, path string) *store.Store {
	t.Helper()

	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
	})
	return st
}

func requireContains(t *testing.T, got, want string) {
	t.Helper()

	if !strings.Contains(got, want) {
		t.Fatalf("expected output to contain %q, got:\n%s", want, got)
	}
}
