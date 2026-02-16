package cli

import (
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Pull posts then display digest",
	RunE:  runAction,
}

func runAction(cmd *cobra.Command, args []string) error {
	if err := pullAction(cmd, args); err != nil {
		return err
	}
	return digestAction(cmd, args)
}
