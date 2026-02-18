package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/ppiankov/noisepan/internal/config"
	"github.com/ppiankov/noisepan/internal/store"
	"github.com/spf13/cobra"
)

// execCommandContext allows mocking exec.CommandContext in tests
var execCommandContext = exec.CommandContext

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify read_now posts with Entropia",
	Long: `Runs entropia scan on URLs from read_now posts to display support index 
and verification details. Requires 'entropia' binary in PATH.`,
	RunE: verifyAction,
}

func init() {
	rootCmd.AddCommand(verifyCmd)

	// Reuse digest flags for consistency
	verifyCmd.Flags().StringVar(&digestSince, "since", "", "time window (e.g. 48h)")
	verifyCmd.Flags().BoolVar(&noColor, "no-color", false, "disable ANSI colors")
}

type EntropiaResult struct {
	URL   string        `json:"url"`
	Score EntropiaScore `json:"score"`
}

type EntropiaScore struct {
	Index      int      `json:"index"`
	Confidence string   `json:"confidence"`
	Conflict   bool     `json:"conflict"`
	Signals    []string `json:"signals"`
}

func verifyAction(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load(configDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	db, err := store.Open(cfg.Storage.Path)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Determine time window
	sinceDur := cfg.Digest.Since.Duration
	if digestSince != "" {
		sinceDur, err = time.ParseDuration(digestSince)
		if err != nil {
			return fmt.Errorf("parse --since: %w", err)
		}
	}
	sinceTime := time.Now().Add(-sinceDur)

	ctx := cmd.Context()

	// Get read_now posts in window
	posts, err := db.GetPosts(ctx, sinceTime, "read_now")
	if err != nil {
		return fmt.Errorf("get posts: %w", err)
	}

	fmt.Printf("noisepan verify — %d read_now posts, checking URLs...\n\n", len(posts))
	fmt.Println("--- Verification ---")
	fmt.Println()

	for _, item := range posts {
		printPostHeader(item)

		postURL := strings.TrimSpace(item.Post.URL)
		if postURL == "" {
			fmt.Println("      entropia: skipped (no URL)")
			fmt.Println()
			continue
		}

		// Check for unscannable domains
		if reason := getSkipReason(postURL); reason != "" {
			fmt.Printf("      entropia: skipped (%s)\n", reason)
			fmt.Println()
			continue
		}

		// Run entropia scan
		result, err := runEntropiaScan(ctx, postURL)
		if err != nil {
			// Non-fatal error
			fmt.Printf("      entropia: error (%v)\n", err)
			fmt.Println()
			continue
		}

		printEntropiaResult(result)
		fmt.Println()
	}

	return nil
}

func printPostHeader(item store.PostWithScore) {
	title := item.Post.Snippet
	if idx := strings.Index(title, "\n"); idx != -1 {
		title = title[:idx]
	}
	if len(title) > 60 {
		title = title[:57] + "..."
	}

	fmt.Printf("  [%d] %s — %s\n", item.Score.Score, item.Post.Channel, title)
	if item.Post.URL != "" {
		fmt.Printf("      %s\n", item.Post.URL)
	}
}

func getSkipReason(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "invalid URL"
	}
	host := strings.ToLower(u.Host)
	if strings.Contains(host, "reddit.com") {
		return "reddit.com not scannable"
	}
	if strings.Contains(host, "t.me") {
		return "t.me requires auth"
	}
	return ""
}

func runEntropiaScan(ctx context.Context, targetURL string) (*EntropiaResult, error) {
	// 30s timeout per scan
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := execCommandContext(ctx, "entropia", "scan", targetURL, "--json")
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("timeout")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("failed: %s", string(exitErr.Stderr))
		}
		return nil, err
	}

	var result EntropiaResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	return &result, nil
}

func printEntropiaResult(res *EntropiaResult) {
	conflictStatus := ", no conflict"
	if res.Score.Conflict {
		conflictStatus = ", ⚠ conflict detected"
	}
	
	fmt.Printf("      entropia: support %d/100, confidence %s%s\n", 
		res.Score.Index, res.Score.Confidence, conflictStatus)
}
