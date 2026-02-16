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
	rootCmd.AddCommand(versionCmd)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
