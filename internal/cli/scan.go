package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newScanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scan [path]",
		Short: "Scan, show a plan, and prompt for confirmation",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runScan,
	}
}

func runScan(cmd *cobra.Command, _ []string) error {
	fmt.Fprintln(cmd.ErrOrStderr(), "scan: not implemented yet")
	return nil
}
