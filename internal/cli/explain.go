package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ppiankov/noisepan/internal/config"
	"github.com/ppiankov/noisepan/internal/store"
	"github.com/ppiankov/noisepan/internal/taste"
	"github.com/spf13/cobra"
)

var explainCmd = &cobra.Command{
	Use:   "explain <post-id>",
	Short: "Show scoring breakdown for a post",
	Args:  cobra.ExactArgs(1),
	RunE:  explainAction,
}

func explainAction(_ *cobra.Command, args []string) error {
	postID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid post ID: %w", err)
	}

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

	ctx := context.Background()

	// Get all posts (no time filter) and find by ID
	posts, err := db.GetPosts(ctx, time.Time{}, "")
	if err != nil {
		return fmt.Errorf("get posts: %w", err)
	}

	var found *store.PostWithScore
	for i := range posts {
		if posts[i].Post.ID == postID {
			found = &posts[i]
			break
		}
	}

	if found == nil {
		return fmt.Errorf("post %d not found", postID)
	}

	p := found.Post
	fmt.Printf("Post #%d\n", p.ID)
	fmt.Printf("  Source:  %s/%s\n", p.Source, p.Channel)
	fmt.Printf("  Snippet: %s\n", p.Snippet)
	if p.URL != "" {
		fmt.Printf("  URL:     %s\n", p.URL)
	}
	fmt.Println()

	// Use stored score if available, otherwise score live
	if found.Score != nil {
		fmt.Printf("Score: %d  Tier: %s\n", found.Score.Score, found.Score.Tier)
		if len(found.Score.Labels) > 0 {
			fmt.Printf("Labels: %v\n", found.Score.Labels)
		}
		fmt.Println()

		if len(found.Score.Explanation) > 0 {
			var contributions []taste.ScoreContribution
			if err := json.Unmarshal(found.Score.Explanation, &contributions); err == nil {
				fmt.Println("Breakdown:")
				for _, c := range contributions {
					fmt.Printf("  %+d  %s\n", c.Points, c.Reason)
				}
			}
		}
	} else {
		sp := taste.Score(storePostToSourcePost(p), profile)
		fmt.Printf("Score: %d  Tier: %s  (not saved)\n", sp.Score, sp.Tier)
		if len(sp.Labels) > 0 {
			fmt.Printf("Labels: %v\n", sp.Labels)
		}
		fmt.Println()
		fmt.Println("Breakdown:")
		for _, c := range sp.Explanation {
			fmt.Printf("  %+d  %s\n", c.Points, c.Reason)
		}
	}

	return nil
}
