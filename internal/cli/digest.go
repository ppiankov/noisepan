package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
	digestSince   string
	digestFormat  string
	digestSource  string
	digestChannel string
	noColor       bool
	digestOutput  string
	digestWebhook string
)

var digestCmd = &cobra.Command{
	Use:   "digest",
	Short: "Score, summarize, and display posts",
	RunE:  digestAction,
}

func init() {
	digestCmd.Flags().StringVar(&digestSince, "since", "", "time window (e.g. 48h)")
	digestCmd.Flags().StringVar(&digestFormat, "format", "", "output format: terminal, json, markdown")
	digestCmd.Flags().StringVar(&digestSource, "source", "", "filter by source (e.g. rss, telegram, reddit)")
	digestCmd.Flags().StringVar(&digestChannel, "channel", "", "filter by channel name")
	digestCmd.Flags().BoolVar(&noColor, "no-color", false, "disable ANSI colors")
	digestCmd.Flags().StringVar(&digestOutput, "output", "", "write digest to file (- for stdout)")
	digestCmd.Flags().StringVar(&digestWebhook, "webhook", "", "POST digest JSON to URL")
}

func digestAction(cmd *cobra.Command, _ []string) error {
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

	ctx := cmd.Context()

	// Get all posts in window
	filter := store.PostFilter{Source: digestSource, Channel: digestChannel}
	posts, err := db.GetPosts(ctx, sinceTime, "", filter)
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

	// Apply digest limits (top_n for read_now, include_skims for skim)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})
	var limited []digest.DigestItem
	readNowCount, skimCount := 0, 0
	for _, item := range items {
		switch item.Tier {
		case taste.TierReadNow:
			if readNowCount < cfg.Digest.TopN {
				limited = append(limited, item)
				readNowCount++
			}
		case taste.TierSkim:
			if skimCount < cfg.Digest.IncludeSkims {
				limited = append(limited, item)
				skimCount++
			}
		default:
			limited = append(limited, item)
		}
	}
	items = limited

	// Detect trending topics across channels
	var scoredPosts []taste.ScoredPost
	for _, item := range items {
		scoredPosts = append(scoredPosts, item.ScoredPost)
	}
	trending := taste.FindTrending(scoredPosts, profile, 3)

	input := digest.DigestInput{
		Items:      items,
		Trending:   trending,
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

	// Determine output writer
	w := os.Stdout
	if digestOutput != "" && digestOutput != "-" {
		dir := filepath.Dir(digestOutput)
		if dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("create output dir: %w", err)
			}
		}
		f, err := os.Create(digestOutput)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer func() { _ = f.Close() }()
		w = f
	}

	if err := formatter.Format(w, input); err != nil {
		return err
	}

	// Webhook: always POST as JSON regardless of --format
	if digestWebhook != "" {
		if err := postWebhook(digestWebhook, input); err != nil {
			fmt.Fprintf(os.Stderr, "warning: webhook failed: %v\n", err)
		}
	}

	return nil
}

func postWebhook(url string, input digest.DigestInput) error {
	jsonFormatter := digest.NewJSON()
	var buf bytes.Buffer
	if err := jsonFormatter.Format(&buf, input); err != nil {
		return fmt.Errorf("format json: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", &buf)
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
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
