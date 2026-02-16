// Package cli provides the command-line interface for noisepan.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version and Commit are set via ldflags at build time.
var (
	Version = "dev"
	Commit  = "none"
)

var configDir string

var rootCmd = &cobra.Command{
	Use:   "noisepan",
	Short: "Extract signal from noisy information streams",
	Long:  "noisepan reads Telegram channels, RSS feeds, and other sources, scores posts by relevance, and produces a concise terminal digest.",
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("noisepan %s (%s)\n", Version, Commit)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configDir, "config", ".noisepan", "config directory")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(digestCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(explainCmd)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
