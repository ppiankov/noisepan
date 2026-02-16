package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ppiankov/noisepan/internal/config"
	"github.com/ppiankov/noisepan/internal/digest"
	"github.com/ppiankov/noisepan/internal/source"
	"github.com/ppiankov/noisepan/internal/store"
	"github.com/ppiankov/noisepan/internal/summarize"
	"github.com/ppiankov/noisepan/internal/taste"
	"github.com/spf13/cobra"
)

var (
	digestSince  string
	digestFormat string
	noColor      bool
)

var digestCmd = &cobra.Command{
	Use:   "digest",
	Short: "Score, summarize, and display posts",
	RunE:  digestAction,
}

func init() {
	digestCmd.Flags().StringVar(&digestSince, "since", "", "time window (e.g. 48h)")
	digestCmd.Flags().StringVar(&digestFormat, "format", "", "output format: terminal, json, markdown")
	digestCmd.Flags().BoolVar(&noColor, "no-color", false, "disable ANSI colors")
}

func digestAction(_ *cobra.Command, _ []string) error {
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

	// Determine time window
	sinceDur := cfg.Digest.Since.Duration
	if digestSince != "" {
		sinceDur, err = time.ParseDuration(digestSince)
		if err != nil {
			return fmt.Errorf("parse --since: %w", err)
		}
	}
	sinceTime := time.Now().Add(-sinceDur)

	ctx := context.Background()

	// Get all posts in window
	posts, err := db.GetPosts(ctx, sinceTime, "")
	if err != nil {
		return fmt.Errorf("get posts: %w", err)
	}

	// Score unscored posts
	now := time.Now()
	for i := range posts {
		if posts[i].Score != nil {
			continue
		}
		sp := taste.Score(storePostToSourcePost(posts[i].Post), profile)
		explanation, _ := json.Marshal(sp.Explanation)

		storeScore := store.Score{
			PostID:      posts[i].Post.ID,
			Score:       sp.Score,
			Labels:      sp.Labels,
			Tier:        sp.Tier,
			ScoredAt:    now,
			Explanation: explanation,
		}
		if err := db.SaveScore(ctx, storeScore); err != nil {
			return fmt.Errorf("save score: %w", err)
		}

		posts[i].Score = &storeScore
	}

	// Build summarizers
	heuristic := &summarize.HeuristicSummarizer{}
	var llmSummarizer summarize.Summarizer
	if cfg.Summarize.Mode == "llm" && cfg.Summarize.LLM.APIKey != "" {
		maxTokens := cfg.Summarize.LLM.MaxTokensPerPost
		if maxTokens == 0 {
			maxTokens = 200
		}
		llmSummarizer = summarize.NewLLM(
			cfg.Summarize.LLM.APIKey,
			cfg.Summarize.LLM.Model,
			maxTokens,
			heuristic,
		)
	}

	// Build digest items
	channels := make(map[string]bool)
	var items []digest.DigestItem

	for _, pws := range posts {
		channels[pws.Post.Channel] = true

		text := pws.Post.Text
		if text == "" {
			text = pws.Post.Snippet
		}

		scored := taste.ScoredPost{
			Post:  storePostToSourcePost(pws.Post),
			Score: pws.Score.Score,
			Tier:  pws.Score.Tier,
		}
		if pws.Score.Labels != nil {
			scored.Labels = pws.Score.Labels
		}

		// Use LLM for read_now posts, heuristic for everything else
		var summer summarize.Summarizer = heuristic
		if llmSummarizer != nil && pws.Score.Tier == taste.TierReadNow {
			summer = llmSummarizer
		}

		items = append(items, digest.DigestItem{
			ScoredPost: scored,
			Summary:    summer.Summarize(text),
		})
	}

	// Populate "also in" annotations
	var postIDs []int64
	for _, pws := range posts {
		postIDs = append(postIDs, pws.Post.ID)
	}
	alsoInMap, err := db.GetAlsoIn(ctx, postIDs)
	if err != nil {
		return fmt.Errorf("get also_in: %w", err)
	}
	for i, pws := range posts {
		if channels, ok := alsoInMap[pws.Post.ID]; ok {
			items[i].AlsoIn = channels
		}
	}

	input := digest.DigestInput{
		Items:      items,
		Channels:   len(channels),
		TotalPosts: len(posts),
		Since:      sinceDur,
	}

	var formatter digest.Formatter
	switch digestFormat {
	case "json":
		formatter = digest.NewJSON()
	case "markdown", "md":
		formatter = digest.NewMarkdown()
	case "terminal", "":
		formatter = digest.NewTerminal(!noColor)
	default:
		return fmt.Errorf("unknown format %q (want terminal, json, or markdown)", digestFormat)
	}
	return formatter.Format(os.Stdout, input)
}

func storePostToSourcePost(p store.Post) source.Post {
	text := p.Text
	if text == "" {
		text = p.Snippet
	}
	return source.Post{
		Source:     p.Source,
		Channel:    p.Channel,
		ExternalID: p.ExternalID,
		Text:       text,
		URL:        p.URL,
		PostedAt:   p.PostedAt,
	}
}
