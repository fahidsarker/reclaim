package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/fahid/reclaim/internal/plan"
)

func newExplainCmd(f *sharedFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "explain <path>",
		Short: "Explain why a path is or isn't a reclaim candidate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ensureUserSpecs(cmd, f)
			ex, err := plan.ExplainPath(args[0], plan.ExplainOptions{
				Aggressive:   f.aggressive,
				UseGitBinary: f.useGitBinary,
				NoConfig:     f.noConfig,
			})
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), plan.FormatExplanation(ex))
			return nil
		},
	}
}
