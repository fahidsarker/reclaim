package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newPlanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "plan [path]",
		Short: "Scan and print a plan without prompting",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runPlan,
	}
}

func runPlan(cmd *cobra.Command, _ []string) error {
	fmt.Fprintln(cmd.ErrOrStderr(), "plan: not implemented yet")
	return nil
}
