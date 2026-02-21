package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ppiankov/noisepan/internal/config"
	"github.com/ppiankov/noisepan/internal/store"
	"github.com/ppiankov/noisepan/internal/taste"
	"github.com/spf13/cobra"
)

var rescoreSince string

var rescoreCmd = &cobra.Command{
	Use:   "rescore",
	Short: "Recompute scores for all posts using current taste profile",
	RunE:  rescoreAction,
}

func init() {
	rescoreCmd.Flags().StringVar(&rescoreSince, "since", "", "time window (e.g. 7d, 48h)")
	rootCmd.AddCommand(rescoreCmd)
}

func rescoreAction(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load(configDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	tastePath := filepath.Join(configDir, config.DefaultTasteFile)
	profile, err := config.LoadTaste(tastePath)
	if err != nil {
		return fmt.Errorf("load taste: %w", err)
	}

	db, err := store.Open(cfg.Storage.Path)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = db.Close() }()

	ctx := cmd.Context()

	// Delete existing scores
	deleted, err := db.DeleteAllScores(ctx)
	if err != nil {
		return fmt.Errorf("delete scores: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Deleted %d existing scores\n", deleted)

	// Determine time window
	sinceDur := cfg.Digest.Since.Duration
	if rescoreSince != "" {
		sinceDur, err = parseDuration(rescoreSince)
		if err != nil {
			return fmt.Errorf("parse --since: %w", err)
		}
	}
	sinceTime := time.Now().Add(-sinceDur)

	// Get all posts in window (all unscored now)
	posts, err := db.GetPosts(ctx, sinceTime, "")
	if err != nil {
		return fmt.Errorf("get posts: %w", err)
	}

	// Re-score each post
	now := time.Now()
	for _, pws := range posts {
		sp := taste.Score(storePostToSourcePost(pws.Post), profile)
		explanation, _ := json.Marshal(sp.Explanation)

		storeScore := store.Score{
			PostID:      pws.Post.ID,
			Score:       sp.Score,
			Labels:      sp.Labels,
			Tier:        sp.Tier,
			ScoredAt:    now,
			Explanation: explanation,
		}
		if err := db.SaveScore(ctx, storeScore); err != nil {
			return fmt.Errorf("save score for post %d: %w", pws.Post.ID, err)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Rescored %d posts\n", len(posts))
	return nil
}
