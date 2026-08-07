package cli

import (
	"github.com/spf13/cobra"
)

// Execute runs the reclaim root command.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reclaim",
		Short: "Reclaim disk space from regenerable project artifacts",
		Long:  "reclaim walks a directory tree, finds regenerable build artifacts and dependency caches, and reclaims the disk space they occupy.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScan(cmd, args)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(newScanCmd())
	cmd.AddCommand(newPlanCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}
