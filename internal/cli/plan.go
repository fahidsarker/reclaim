package cli

import (
	"github.com/spf13/cobra"
)

func newPlanCmd(f *sharedFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "plan [path]",
		Short: "Scan and print a plan without prompting",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlan(cmd, args, f)
		},
	}
}

func runPlan(cmd *cobra.Command, args []string, f *sharedFlags) error {
	return runPlanDry(cmd, args, f)
}
