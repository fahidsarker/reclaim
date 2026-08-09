package cli

import (
	"github.com/spf13/cobra"

	"github.com/fahid/reclaim/internal/plan"
	"github.com/fahid/reclaim/internal/scan"
)

func newScanCmd(f *sharedFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "scan [path]",
		Short: "Scan, show a plan, and prompt for confirmation",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScan(cmd, args, f)
		},
	}
}

func runScan(cmd *cobra.Command, args []string, f *sharedFlags) error {
	if !f.dryRun {
		return errExecuteUnavailable()
	}
	return runPlanDry(cmd, args, f)
}

func runPlanDry(cmd *cobra.Command, args []string, f *sharedFlags) error {
	root := resolvePath(args)
	res, err := scan.Walk(scan.Options{
		Root:             root,
		MaxDepth:         f.depth,
		Concurrency:      f.concurrency,
		IKnowWhatImDoing: f.iKnowWhatImDoing,
		NoConfig:         f.noConfig,
	})
	if err != nil {
		return err
	}

	p := plan.Build(res)
	return plan.WriteHuman(cmd.OutOrStdout(), p)
}
