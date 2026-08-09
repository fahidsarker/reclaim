package cli

import (
	"errors"
	"runtime"

	"github.com/spf13/cobra"
)

// sharedFlags holds flags shared by scan and plan.
type sharedFlags struct {
	depth            int
	concurrency      int
	dryRun           bool
	yes              bool
	toTrash          bool
	noSize           bool
	noColor          bool
	iKnowWhatImDoing bool
	noConfig         bool
	aggressive       bool
	json             bool
	followSymlinks   bool
	crossDevice      bool
	useGitBinary     bool
	quiet            bool
	verbose          int
	frameworks       []string
	excludeFramework []string
	exclude          []string
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
	pf.BoolVarP(&f.yes, "yes", "y", false, "skip the confirmation prompt")
	pf.BoolVar(&f.toTrash, "to-trash", false, "send targets to the OS trash instead of permanent delete")
	pf.BoolVar(&f.noSize, "no-size", false, "skip size computation")
	pf.BoolVar(&f.noColor, "no-color", false, "disable coloured output")
	pf.BoolVar(&f.iKnowWhatImDoing, "i-know-what-im-doing", false, "allow scanning / or $HOME")
	pf.BoolVar(&f.noConfig, "no-config", false, "ignore all .reclaim.yaml and global config")
	pf.BoolVar(&f.aggressive, "aggressive", false, "include SafetyRequiresFlag targets (Pods, venv, .terraform, …)")
	pf.BoolVar(&f.json, "json", false, "machine-readable plan to stdout")
	pf.BoolVar(&f.followSymlinks, "follow-symlinks", false, "traverse symlinked directories (cycle-guarded)")
	pf.BoolVar(&f.crossDevice, "cross-device", false, "cross filesystem boundaries")
	pf.BoolVar(&f.useGitBinary, "use-git-binary", false, "use git check-ignore for ignore matching")
	pf.BoolVarP(&f.quiet, "quiet", "q", false, "errors only")
	pf.CountVarP(&f.verbose, "verbose", "v", "per-decision reasoning (repeatable: -vv)")
	pf.StringSliceVarP(&f.frameworks, "framework", "f", nil, "restrict to named detectors (repeatable)")
	pf.StringSliceVar(&f.excludeFramework, "exclude-framework", nil, "denylist detectors (repeatable)")
	pf.StringSliceVar(&f.exclude, "exclude", nil, "skip paths matching a glob (repeatable)")

	cmd.AddCommand(newScanCmd(f))
	cmd.AddCommand(newPlanCmd(f))
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newExplainCmd(f))
	cmd.AddCommand(newInitCmd(f))
	cmd.AddCommand(newDetectorsCmd(f))

	return cmd
}

func resolvePath(args []string) string {
	if len(args) == 0 {
		return "."
	}
	return args[0]
}

// ExitCode returns the process exit code for err, defaulting to 1.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	return 1
}
