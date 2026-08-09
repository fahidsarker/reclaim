package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/fahid/reclaim/internal/config"
	"github.com/fahid/reclaim/internal/detect"
	"github.com/fahid/reclaim/internal/exec"
	"github.com/fahid/reclaim/internal/plan"
	"github.com/fahid/reclaim/internal/scan"
	"github.com/fahid/reclaim/internal/ui"
)

var loadUserSpecsOnce sync.Once

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
	p, err := buildSizedPlan(cmd, args, f)
	if err != nil {
		return err
	}

	if err := emitPlan(cmd, p, f); err != nil {
		return err
	}

	if f.dryRun {
		return nil
	}

	if !f.yes {
		promptOut := cmd.OutOrStdout()
		if f.json {
			promptOut = cmd.ErrOrStderr()
		}
		ok, err := ui.Confirm(cmd.InOrStdin(), promptOut)
		if err != nil {
			return err
		}
		if !ok {
			return exitErrorf(3, "aborted")
		}
	}

	return runExecute(cmd, p, f)
}

func runExecute(cmd *cobra.Command, p *plan.Plan, f *sharedFlags) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	res, err := exec.Run(p, exec.Options{
		Root:    p.Root,
		ToTrash: f.toTrash,
		Yes:     f.yes,
		Context: ctx,
		Warn:    cmd.ErrOrStderr(),
	})
	if err != nil {
		return err
	}

	summary := exec.SummaryLine(res)
	out := cmd.OutOrStdout()
	if f.json {
		out = cmd.ErrOrStderr()
	}
	if summary != "" {
		fmt.Fprintln(out, summary)
	}

	if res.Failed() {
		return exitErrorf(4, "%s", summary)
	}
	return nil
}

func runPlanDry(cmd *cobra.Command, args []string, f *sharedFlags) error {
	p, err := buildSizedPlan(cmd, args, f)
	if err != nil {
		return err
	}
	return emitPlan(cmd, p, f)
}

func emitPlan(cmd *cobra.Command, p *plan.Plan, f *sharedFlags) error {
	if f.json {
		return ui.WriteJSON(cmd.OutOrStdout(), p, ui.JSONOptions{NoSize: f.noSize})
	}
	if f.quiet {
		return nil
	}
	if err := ui.Render(cmd.OutOrStdout(), p, ui.RenderOptions{
		NoSize:  f.noSize,
		NoColor: f.noColor,
		Quiet:   f.quiet,
	}); err != nil {
		return err
	}
	if f.verbose >= 1 {
		writeVerboseDecisions(cmd.ErrOrStderr(), p)
	}
	return nil
}

func writeVerboseDecisions(w io.Writer, p *plan.Plan) {
	for _, d := range p.Decisions {
		proj := ""
		if d.Project != nil {
			proj = d.Project.Framework
		}
		fmt.Fprintf(w, "verbose: %s verdict=%s reason=%q framework=%s\n",
			d.Target.Path, d.Verdict, d.Reason, proj)
	}
}

func buildSizedPlan(cmd *cobra.Command, args []string, f *sharedFlags) (*plan.Plan, error) {
	ensureUserSpecs(cmd, f)

	root := resolvePath(args)
	start := time.Now()
	warn := func(string) {}
	if f.verbose >= 2 && !f.quiet {
		warn = func(msg string) {
			fmt.Fprintln(cmd.ErrOrStderr(), "verbose:", msg)
		}
	}
	res, err := scan.Walk(scan.Options{
		Root:             root,
		MaxDepth:         f.depth,
		Concurrency:      f.concurrency,
		FollowSymlinks:   f.followSymlinks,
		CrossDevice:      f.crossDevice,
		Exclude:          f.exclude,
		Frameworks:       f.frameworks,
		ExcludeFramework: f.excludeFramework,
		IKnowWhatImDoing: f.iKnowWhatImDoing,
		NoConfig:         f.noConfig,
		Warn:             warn,
	})
	if err != nil {
		return nil, err
	}
	scanDur := time.Since(start)

	p := plan.Build(res, plan.Options{
		Aggressive:       f.aggressive,
		UseGitBinary:     f.useGitBinary,
		Frameworks:       f.frameworks,
		ExcludeFramework: f.excludeFramework,
	})
	p.Stats.DirsWalked = res.DirsWalked
	p.Stats.Projects = len(res.Projects)
	p.Stats.Depth = f.depth
	p.Stats.ScanDuration = scanDur

	if err := plan.Size(p, plan.SizeOptions{
		Concurrency: f.concurrency,
		NoSize:      f.noSize,
	}); err != nil {
		return nil, err
	}

	// Ensure absolute root for display when walk resolved it.
	if res.Root != "" {
		p.Root = res.Root
	}
	return p, nil
}

func ensureUserSpecs(cmd *cobra.Command, f *sharedFlags) {
	if f.noConfig {
		return
	}
	loadUserSpecsOnce.Do(func() {
		dir, err := config.DefaultSpecsDir()
		if err != nil {
			return
		}
		detect.LoadUserSpecsFromDir(dir, func(msg string) {
			if f.quiet {
				return
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "warning:", msg)
		})
	})
}
