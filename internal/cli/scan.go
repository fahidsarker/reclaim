package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"

	"github.com/fahid/reclaim/internal/exec"
	"github.com/fahid/reclaim/internal/plan"
	"github.com/fahid/reclaim/internal/scan"
	"github.com/fahid/reclaim/internal/ui"
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
	p, err := buildSizedPlan(cmd, args, f)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if err := ui.Render(out, p, ui.RenderOptions{NoSize: f.noSize, NoColor: f.noColor}); err != nil {
		return err
	}

	if f.dryRun {
		return nil
	}

	if !f.yes {
		ok, err := ui.Confirm(cmd.InOrStdin(), out)
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
	if summary != "" {
		fmt.Fprintln(cmd.OutOrStdout(), summary)
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
	return ui.Render(cmd.OutOrStdout(), p, ui.RenderOptions{NoSize: f.noSize, NoColor: f.noColor})
}

func buildSizedPlan(cmd *cobra.Command, args []string, f *sharedFlags) (*plan.Plan, error) {
	root := resolvePath(args)
	start := time.Now()
	res, err := scan.Walk(scan.Options{
		Root:             root,
		MaxDepth:         f.depth,
		Concurrency:      f.concurrency,
		IKnowWhatImDoing: f.iKnowWhatImDoing,
		NoConfig:         f.noConfig,
	})
	if err != nil {
		return nil, err
	}
	scanDur := time.Since(start)

	p := plan.Build(res)
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
