package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// sharedFlags holds flags shared by scan and plan.
type sharedFlags struct {
	depth            int
	concurrency      int
	dryRun           bool
	iKnowWhatImDoing bool
}

func defaultConcurrency() int {
	n := runtime.NumCPU()
	if n > 8 {
		return 8
	}
	if n < 1 {
		return 1
	}
	return n
}

// Execute runs the reclaim root command.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	f := &sharedFlags{
		depth:       8,
		concurrency: defaultConcurrency(),
	}

	cmd := &cobra.Command{
		Use:   "reclaim",
		Short: "Reclaim disk space from regenerable project artifacts",
		Long:  "reclaim walks a directory tree, finds regenerable build artifacts and dependency caches, and reclaims the disk space they occupy.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScan(cmd, args, f)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	pf := cmd.PersistentFlags()
	pf.IntVarP(&f.depth, "depth", "d", 8, "max directory depth below root")
	pf.IntVar(&f.concurrency, "concurrency", defaultConcurrency(), "walker parallelism")
	pf.BoolVarP(&f.dryRun, "dry-run", "n", false, "print the plan and exit without deleting")
	pf.BoolVar(&f.iKnowWhatImDoing, "i-know-what-im-doing", false, "allow scanning / or $HOME")

	cmd.AddCommand(newScanCmd(f))
	cmd.AddCommand(newPlanCmd(f))
	cmd.AddCommand(newVersionCmd())

	return cmd
}

func resolvePath(args []string) string {
	if len(args) == 0 {
		return "."
	}
	return args[0]
}

func errExecuteUnavailable() error {
	return fmt.Errorf("deletion is not implemented yet; use `reclaim plan` or `reclaim scan --dry-run` to list candidates")
}
