package cli

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/ppiankov/noisepan/internal/config"
	"github.com/ppiankov/noisepan/internal/store"
	"github.com/spf13/cobra"
)

var statsSince string

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show feed and scoring analytics",
	RunE:  statsAction,
}

func init() {
	statsCmd.Flags().StringVar(&statsSince, "since", "30d", "time window (e.g. 7d, 48h)")
	rootCmd.AddCommand(statsCmd)
}

const staleDays = 7

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
		fmt.Fprintln(os.Stdout, "No posts found. Run 'noisepan pull' first.")
		return nil
	}

	printStats(os.Stdout, stats, sinceDur)
	return nil
}

func printStats(w *os.File, stats []store.ChannelStats, since time.Duration) {
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
		fmt.Fprintf(w, "  %-*s  %5d  %8d  %4d  %7d  %5.0f%%\n",
			maxChan, name, cs.Total, cs.ReadNow, cs.Skim, cs.Ignored, signalPct(cs))
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
	now := time.Now()
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
