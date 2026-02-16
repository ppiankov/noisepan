package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
		printCheck(true, "config.yaml (%d telegram channels, %d rss feeds)",
			len(cfg.Sources.Telegram.Channels), len(cfg.Sources.RSS.Feeds))
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
	if cfg != nil {
		db, err := store.Open(cfg.Storage.Path)
		if err != nil {
			printCheck(false, "database: %v", err)
			ok = false
		} else {
			_ = db.Close()
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

	if !ok {
		return fmt.Errorf("some checks failed")
	}
	fmt.Println("\nAll checks passed.")
	return nil
}

func printCheck(pass bool, format string, args ...any) {
	mark := "FAIL"
	if pass {
		mark = " OK "
	}
	fmt.Printf("[%s] %s\n", mark, fmt.Sprintf(format, args...))
}
