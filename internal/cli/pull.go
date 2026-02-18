package cli

import (
	"fmt"
	"path/filepath"
	"regexp"
	"time"

	"github.com/ppiankov/noisepan/internal/config"
	"github.com/ppiankov/noisepan/internal/privacy"
	"github.com/ppiankov/noisepan/internal/source"
	"github.com/ppiankov/noisepan/internal/store"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Fetch posts from all configured sources",
	RunE:  pullAction,
}

func pullAction(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load(configDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	db, err := store.Open(cfg.Storage.Path)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = db.Close() }()

	since := time.Now().Add(-cfg.Digest.Since.Duration)
	ctx := cmd.Context()

	// Build sources
	var sources []source.Source

	if len(cfg.Sources.Telegram.Channels) > 0 {
		scriptPath := cfg.Sources.Telegram.Script
		if scriptPath == "" {
			scriptPath = filepath.Join(configDir, "..", "scripts", "collector_telegram.py")
		}
		tg, err := source.NewTelegram(
			scriptPath,
			cfg.Sources.Telegram.PythonPath,
			cfg.Sources.Telegram.APIID,
			cfg.Sources.Telegram.APIHash,
			cfg.Sources.Telegram.SessionDir,
			cfg.Sources.Telegram.Channels,
		)
		if err != nil {
			return fmt.Errorf("create telegram source: %w", err)
		}
		sources = append(sources, tg)
	}

	if len(cfg.Sources.RSS.Feeds) > 0 {
		rs, err := source.NewRSS(cfg.Sources.RSS.Feeds)
		if err != nil {
			return fmt.Errorf("create rss source: %w", err)
		}
		sources = append(sources, rs)
	}

	if len(cfg.Sources.Reddit.Subreddits) > 0 {
		rd, err := source.NewReddit(cfg.Sources.Reddit.Subreddits)
		if err != nil {
			return fmt.Errorf("create reddit source: %w", err)
		}
		sources = append(sources, rd)
	}

	if cfg.Sources.ForgePlan.Script != "" {
		fp, err := source.NewForgePlan(cfg.Sources.ForgePlan.Script)
		if err != nil {
			return fmt.Errorf("create forgeplan source: %w", err)
		}
		sources = append(sources, fp)
	}

	// Compile redact patterns if enabled
	var redactPatterns []*regexp.Regexp
	if cfg.Privacy.Redact.Enabled && len(cfg.Privacy.Redact.Patterns) > 0 {
		redactPatterns, err = privacy.Compile(cfg.Privacy.Redact.Patterns)
		if err != nil {
			return fmt.Errorf("compile redact patterns: %w", err)
		}
	}

	totalInserted := 0
	channels := make(map[string]bool)

	for _, src := range sources {
		posts, err := src.Fetch(since)
		if err != nil {
			fmt.Printf("warning: %s: %v\n", src.Name(), err)
			continue
		}

		now := time.Now()
		for _, p := range posts {
			channels[p.Channel] = true

			text := p.Text

			// Apply redaction before snippet extraction
			if len(redactPatterns) > 0 {
				text = privacy.Apply(text, redactPatterns)
			}

			// Generate snippet from (possibly redacted) text, then
			// clear full text if store_full_text is false.
			snippet := ""
			storeText := text
			if !cfg.Privacy.StoreFullText {
				snippet = firstNRunes(text, 200)
				storeText = ""
			}

			_, err := db.InsertPost(ctx, store.PostInput{
				Source:     p.Source,
				Channel:    p.Channel,
				ExternalID: p.ExternalID,
				Text:       storeText,
				Snippet:    snippet,
				URL:        p.URL,
				PostedAt:   p.PostedAt,
				FetchedAt:  now,
			})
			if err != nil {
				return fmt.Errorf("insert post: %w", err)
			}
			totalInserted++
		}
	}

	dupes, err := db.Deduplicate(ctx)
	if err != nil {
		return fmt.Errorf("deduplicate: %w", err)
	}

	pruned, err := db.PruneOld(ctx, cfg.Storage.RetainDays)
	if err != nil {
		return fmt.Errorf("prune old: %w", err)
	}

	fmt.Printf("Pulled %d posts from %d channels", totalInserted, len(channels))
	if dupes > 0 {
		fmt.Printf(" (%d duplicates removed)", dupes)
	}
	if pruned > 0 {
		fmt.Printf(" (%d old posts pruned)", pruned)
	}
	fmt.Println()

	return nil
}

func firstNRunes(s string, n int) string {
	if n <= 0 || s == "" {
		return ""
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}
