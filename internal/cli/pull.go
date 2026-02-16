package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ppiankov/noisepan/internal/config"
	"github.com/ppiankov/noisepan/internal/source"
	"github.com/ppiankov/noisepan/internal/store"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Fetch posts from all configured sources",
	RunE:  pullAction,
}

func pullAction(_ *cobra.Command, _ []string) error {
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
	ctx := context.Background()

	// Build sources
	var sources []source.Source

	if len(cfg.Sources.Telegram.Channels) > 0 {
		scriptPath := filepath.Join(configDir, "..", "scripts", "collector_telegram.py")
		tg, err := source.NewTelegram(
			scriptPath,
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
			_, err := db.InsertPost(ctx, store.PostInput{
				Source:     p.Source,
				Channel:    p.Channel,
				ExternalID: p.ExternalID,
				Text:       p.Text,
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

	fmt.Printf("Pulled %d posts from %d channels", totalInserted, len(channels))
	if dupes > 0 {
		fmt.Printf(" (%d duplicates removed)", dupes)
	}
	fmt.Println()

	return nil
}
