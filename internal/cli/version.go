package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is the reclaim version string. Overridden via ldflags in release builds.
var Version = "0.1.0-dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the reclaim version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), Version)
			return nil
		},
	}
}
