package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/ppiankov/noisepan/internal/config"
	"github.com/ppiankov/noisepan/internal/store"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system health and dependencies",
	RunE:  doctorAction,
}

func doctorAction(_ *cobra.Command, _ []string) error {
	ok := true

	// Config dir
	if info, err := os.Stat(configDir); err != nil || !info.IsDir() {
		printCheck(false, "config directory %s", configDir)
		ok = false
	} else {
		printCheck(true, "config directory %s", configDir)
	}

	// Config file
	cfg, err := config.Load(configDir)
	if err != nil {
		printCheck(false, "config.yaml: %v", err)
		ok = false
	} else {
		extras := ""
		if cfg.Sources.HN.MinPoints > 0 {
			extras += ", hn"
		}
		if cfg.Sources.ForgePlan.Script != "" {
			extras += ", forgeplan"
		}
		printCheck(true, "config.yaml (%d telegram channels, %d rss feeds, %d subreddits%s)",
			len(cfg.Sources.Telegram.Channels), len(cfg.Sources.RSS.Feeds), len(cfg.Sources.Reddit.Subreddits), extras)
	}

	// Taste profile
	tastePath := filepath.Join(configDir, config.DefaultTasteFile)
	if _, err := config.LoadTaste(tastePath); err != nil {
		printCheck(false, "taste.yaml: %v", err)
		ok = false
	} else {
		printCheck(true, "taste.yaml")
	}

	// Database
	var db *store.Store
	if cfg != nil {
		db, err = store.Open(cfg.Storage.Path)
		if err != nil {
			printCheck(false, "database: %v", err)
			ok = false
		} else {
			defer func() { _ = db.Close() }()
			printCheck(true, "database %s", cfg.Storage.Path)
		}
	}

	// Python
	if _, err := exec.LookPath("python3"); err != nil {
		printCheck(false, "python3 not found")
		ok = false
	} else {
		printCheck(true, "python3")
	}

	// Telethon
	cmd := exec.Command("python3", "-c", "import telethon")
	if err := cmd.Run(); err != nil {
		printCheck(false, "telethon not installed (pip install telethon)")
		ok = false
	} else {
		printCheck(true, "telethon")
	}

	// Forge-plan script
	if cfg != nil && cfg.Sources.ForgePlan.Script != "" {
		if info, err := os.Stat(cfg.Sources.ForgePlan.Script); err != nil {
			printCheck(false, "forge-plan script: %v", err)
			ok = false
		} else if info.IsDir() {
			printCheck(false, "forge-plan script: %s is a directory", cfg.Sources.ForgePlan.Script)
			ok = false
		} else {
			printCheck(true, "forge-plan script %s", cfg.Sources.ForgePlan.Script)
		}
	}

	// Telegram session
	if cfg != nil && cfg.Sources.Telegram.SessionDir != "" {
		sessionFile := filepath.Join(cfg.Sources.Telegram.SessionDir, "noisepan.session")
		if _, err := os.Stat(sessionFile); err != nil {
			printCheck(false, "telegram session (run collector_telegram.py manually first)")
			ok = false
		} else {
			printCheck(true, "telegram session")
		}
	}

	// Feed health (info-level, non-fatal)
	if db != nil && cfg != nil {
		checkFeedHealth(db, cfg)
	}

	if !ok {
		return fmt.Errorf("some checks failed")
	}
	fmt.Println("\nAll checks passed.")
	return nil
}

func checkFeedHealth(db *store.Store, cfg *config.Config) {
	ctx := context.Background()

	// Look back 30 days for feed health assessment
	since := time.Now().AddDate(0, 0, -30)
	stats, err := db.GetChannelStats(ctx, since)
	if err != nil || len(stats) == 0 {
		return // no data yet, skip
	}

	// Build set of configured channels for comparison
	configuredFeeds := make(map[string]bool)
	for _, feed := range cfg.Sources.RSS.Feeds {
		configuredFeeds[feed] = true
	}
	for _, ch := range cfg.Sources.Telegram.Channels {
		configuredFeeds[ch] = true
	}

	staleThreshold := time.Now().AddDate(0, 0, -staleDays)
	fmt.Println()

	var totalPosts, totalIgnored int
	for _, cs := range stats {
		totalPosts += cs.Total
		totalIgnored += cs.Ignored

		if cs.LastSeen.Before(staleThreshold) {
			daysAgo := int(time.Since(cs.LastSeen).Hours() / 24)
			printInfo("stale: %s — last post %d days ago", cs.Channel, daysAgo)
		}
		if cs.Total >= 5 && cs.Ignored == cs.Total {
			printInfo("all noise: %s — %d posts, all ignored (consider adjusting taste profile)", cs.Channel, cs.Total)
		}
	}

	// Warn if taste profile is too narrow — high ignore rate means important stories may be buried.
	if totalPosts >= 50 {
		ignoreRate := float64(totalIgnored) / float64(totalPosts) * 100
		if ignoreRate > 95 {
			printInfo("blind spot risk: %.0f%% of %d posts ignored — taste profile may be too narrow, important stories could be buried in noise", ignoreRate, totalPosts)
		}
	}
}

func printCheck(pass bool, format string, args ...any) {
	mark := "FAIL"
	if pass {
		mark = " OK "
	}
	fmt.Printf("[%s] %s\n", mark, fmt.Sprintf(format, args...))
}

func printInfo(format string, args ...any) {
	fmt.Printf("[INFO] %s\n", fmt.Sprintf(format, args...))
}
