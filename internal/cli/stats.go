package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/ppiankov/noisepan/internal/config"
	"github.com/ppiankov/noisepan/internal/store"
	"github.com/spf13/cobra"
)

var (
	statsSince  string
	statsFormat string
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show feed and scoring analytics",
	RunE:  statsAction,
}

func init() {
	statsCmd.Flags().StringVar(&statsSince, "since", "30d", "time window (e.g. 7d, 48h)")
	statsCmd.Flags().StringVar(&statsFormat, "format", "terminal", "output format: terminal, json")
	rootCmd.AddCommand(statsCmd)
}

const (
	staleDays         = 7
	maturityThreshold = 30 // days of data needed before stats are reliable
)

func statsAction(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load(configDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	db, err := store.Open(cfg.Storage.Path)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = db.Close() }()

	sinceDur, err := parseDuration(statsSince)
	if err != nil {
		return fmt.Errorf("parse --since: %w", err)
	}
	sinceTime := time.Now().Add(-sinceDur)

	ctx := cmd.Context()

	stats, err := db.GetChannelStats(ctx, sinceTime)
	if err != nil {
		return fmt.Errorf("get stats: %w", err)
	}

	if len(stats) == 0 {
		if statsFormat == "json" {
			fmt.Fprintln(os.Stdout, `{"channels":[],"distribution":{}}`)
			return nil
		}
		fmt.Fprintln(os.Stdout, "No posts found. Run 'noisepan pull' first.")
		return nil
	}

	switch statsFormat {
	case "json":
		return printStatsJSON(os.Stdout, stats, sinceDur)
	case "terminal", "":
		printStats(os.Stdout, stats, sinceDur)
		return nil
	default:
		return fmt.Errorf("unknown format %q (want terminal or json)", statsFormat)
	}
}

type jsonStatsOutput struct {
	Channels     []jsonChannelStats `json:"channels"`
	Distribution jsonDistribution   `json:"distribution"`
}

type jsonChannelStats struct {
	Source   string  `json:"source"`
	Channel  string  `json:"channel"`
	Total    int     `json:"total"`
	ReadNow  int     `json:"read_now"`
	Skim     int     `json:"skim"`
	Ignored  int     `json:"ignored"`
	Signal   float64 `json:"signal_pct"`
	DataDays int     `json:"data_days"`
}

type jsonDistribution struct {
	ReadNow int `json:"read_now"`
	Skim    int `json:"skim"`
	Ignored int `json:"ignored"`
	Total   int `json:"total"`
}

func printStatsJSON(w io.Writer, stats []store.ChannelStats, _ time.Duration) error {
	now := time.Now()
	channels := make([]jsonChannelStats, 0, len(stats))
	dist := jsonDistribution{}

	for _, cs := range stats {
		dataDays := int(now.Sub(cs.FirstSeen).Hours() / 24)
		if dataDays < 1 {
			dataDays = 1
		}
		channels = append(channels, jsonChannelStats{
			Source:   cs.Source,
			Channel:  cs.Channel,
			Total:    cs.Total,
			ReadNow:  cs.ReadNow,
			Skim:     cs.Skim,
			Ignored:  cs.Ignored,
			Signal:   signalPct(cs),
			DataDays: dataDays,
		})
		dist.ReadNow += cs.ReadNow
		dist.Skim += cs.Skim
		dist.Ignored += cs.Ignored
		dist.Total += cs.Total
	}

	out := jsonStatsOutput{
		Channels:     channels,
		Distribution: dist,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func printStats(w *os.File, stats []store.ChannelStats, since time.Duration) {
	now := time.Now()

	totalPosts := 0
	totalReadNow := 0
	totalSkim := 0
	totalIgnored := 0
	for _, cs := range stats {
		totalPosts += cs.Total
		totalReadNow += cs.ReadNow
		totalSkim += cs.Skim
		totalIgnored += cs.Ignored
	}

	sinceStr := formatStatsDuration(since)
	fmt.Fprintf(w, "noisepan stats — %s, %d posts from %d channels\n\n", sinceStr, totalPosts, len(stats))

	// Signal-to-noise by channel, sorted by signal % descending
	sorted := make([]store.ChannelStats, len(stats))
	copy(sorted, stats)
	sort.Slice(sorted, func(i, j int) bool {
		return signalPct(sorted[i]) > signalPct(sorted[j])
	})

	fmt.Fprintln(w, "--- Signal-to-Noise by Channel ---")
	fmt.Fprintln(w)

	// Calculate column width for channel name
	maxChan := 7 // minimum "Channel"
	for _, cs := range sorted {
		name := cs.Channel
		if len(name) > maxChan {
			maxChan = len(name)
		}
	}
	if maxChan > 40 {
		maxChan = 40
	}

	fmt.Fprintf(w, "  %-*s  %5s  %8s  %4s  %7s  %6s\n", maxChan, "Channel", "Posts", "Read Now", "Skim", "Ignored", "Signal")
	for _, cs := range sorted {
		name := cs.Channel
		if len(name) > maxChan {
			name = name[:maxChan-1] + "…"
		}
		signal := fmt.Sprintf("%5.0f%%", signalPct(cs))
		dataDays := int(now.Sub(cs.FirstSeen).Hours() / 24)
		if dataDays < maturityThreshold {
			signal = fmt.Sprintf("%5.0f%% (%dd data)", signalPct(cs), dataDays)
		}
		fmt.Fprintf(w, "  %-*s  %5d  %8d  %4d  %7d  %s\n",
			maxChan, name, cs.Total, cs.ReadNow, cs.Skim, cs.Ignored, signal)
	}
	fmt.Fprintln(w)

	// Scoring distribution
	fmt.Fprintln(w, "--- Scoring Distribution ---")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Read Now:  %5d  (%.1f%%)\n", totalReadNow, pct(totalReadNow, totalPosts))
	fmt.Fprintf(w, "  Skim:      %5d  (%.1f%%)\n", totalSkim, pct(totalSkim, totalPosts))
	fmt.Fprintf(w, "  Ignored:   %5d  (%.1f%%)\n", totalIgnored, pct(totalIgnored, totalPosts))
	fmt.Fprintln(w)

	// Stale channels
	staleThreshold := now.AddDate(0, 0, -staleDays)
	var stale []store.ChannelStats
	for _, cs := range stats {
		if cs.LastSeen.Before(staleThreshold) {
			stale = append(stale, cs)
		}
	}
	if len(stale) > 0 {
		fmt.Fprintf(w, "--- Stale Channels (no posts in %d+ days) ---\n\n", staleDays)
		for _, cs := range stale {
			daysAgo := int(now.Sub(cs.LastSeen).Hours() / 24)
			fmt.Fprintf(w, "  %s — last post %d days ago\n", cs.Channel, daysAgo)
		}
		fmt.Fprintln(w)
	}
}

func signalPct(cs store.ChannelStats) float64 {
	if cs.Total == 0 {
		return 0
	}
	return float64(cs.ReadNow+cs.Skim) / float64(cs.Total) * 100
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

// parseDuration handles both Go durations and "Nd" day notation.
func parseDuration(s string) (time.Duration, error) {
	if len(s) > 1 && s[len(s)-1] == 'd' {
		var days int
		if _, err := fmt.Sscanf(s, "%dd", &days); err == nil && days > 0 {
			return time.Duration(days) * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(s)
}

func formatStatsDuration(d time.Duration) string {
	hours := int(d.Hours())
	if hours >= 24 && hours%24 == 0 {
		return fmt.Sprintf("%d days", hours/24)
	}
	return fmt.Sprintf("%dh", hours)
}
