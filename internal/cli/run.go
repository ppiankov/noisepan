package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	runEvery        string
	runPullAction   = pullAction
	runDigestAction = digestAction
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Pull posts then display digest",
	RunE:  runAction,
}

func init() {
	runCmd.Flags().StringVar(&runEvery, "every", "", "run continuously at interval (e.g. 30m)")
	runCmd.Flags().StringVar(&digestSince, "since", "", "time window (e.g. 48h)")
	runCmd.Flags().StringVar(&digestFormat, "format", "", "output format: terminal, json, markdown")
	runCmd.Flags().StringVar(&digestSource, "source", "", "filter by source")
	runCmd.Flags().StringVar(&digestChannel, "channel", "", "filter by channel name")
	runCmd.Flags().BoolVar(&noColor, "no-color", false, "disable ANSI colors")
	runCmd.Flags().StringVar(&digestOutput, "output", "", "write digest to file")
	runCmd.Flags().StringVar(&digestWebhook, "webhook", "", "POST digest JSON to URL")
}

func runAction(cmd *cobra.Command, args []string) error {
	interval, err := parseRunEvery(runEvery)
	if err != nil {
		return err
	}

	if interval == 0 {
		return runPipeline(cmd, args)
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	ctx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	return runWatch(ctx, interval, func() error {
		return runPipeline(cmd, args)
	})
}

func parseRunEvery(value string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}

	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse --every: %w", err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("--every must be greater than zero")
	}
	return d, nil
}

func runPipeline(cmd *cobra.Command, args []string) error {
	if err := runPullAction(cmd, args); err != nil {
		return err
	}
	return runDigestAction(cmd, args)
}

func runWatch(ctx context.Context, interval time.Duration, runOnce func() error) error {
	if err := runOnce(); err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := runOnce(); err != nil {
				return err
			}
		}
	}
}
