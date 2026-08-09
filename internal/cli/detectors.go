package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fahid/reclaim/internal/detect"
	"github.com/fahid/reclaim/internal/scan"
)

func newDetectorsCmd(f *sharedFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detectors",
		Short: "Inspect registered framework detectors",
	}
	cmd.AddCommand(newDetectorsListCmd(f))
	cmd.AddCommand(newDetectorsShowCmd(f))
	cmd.AddCommand(newDetectorsTestCmd(f))
	return cmd
}

func newDetectorsListCmd(f *sharedFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered detectors and their sources",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ensureUserSpecs(cmd, f)
			detect.MustLoadEmbedded()
			for _, d := range detect.ListDetectorsSorted() {
				desc := detect.DetectorDescription(d)
				src := detect.DetectorSource(d)
				if desc != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%-20s  pri=%-3d  %-8s  %s\n", d.Name(), d.Priority(), src, desc)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%-20s  pri=%-3d  %s\n", d.Name(), d.Priority(), src)
				}
			}
			return nil
		},
	}
}

func newDetectorsShowCmd(f *sharedFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show a resolved detector spec after extends expansion",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ensureUserSpecs(cmd, f)
			detect.MustLoadEmbedded()
			name := args[0]
			d := detect.FindDetector(name)
			if d == nil {
				return exitErrorf(2, "unknown detector %q", name)
			}
			if sd, ok := d.(*detect.SpecDetector); ok {
				out, err := detect.FormatResolvedSpecYAML(sd.Spec)
				if err != nil {
					return err
				}
				fmt.Fprint(cmd.OutOrStdout(), out)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "name: %s\nsource: %s\npriority: %d\nkind: go\ndescription: %s\n",
				d.Name(), detect.DetectorSource(d), d.Priority(), detect.DetectorDescription(d))
			return nil
		},
	}
}

func newDetectorsTestCmd(f *sharedFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "test <dir>",
		Short: "Show which detectors match a directory and why",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ensureUserSpecs(cmd, f)
			detect.MustLoadEmbedded()

			dir, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			st, err := os.Stat(dir)
			if err != nil {
				return err
			}
			if !st.IsDir() {
				return exitErrorf(2, "not a directory: %s", dir)
			}

			cache := scan.NewDirCache()
			ctx := &detect.Context{Cache: cache}
			out := cmd.OutOrStdout()

			for _, d := range detect.ListDetectorsSorted() {
				m, err := d.Detect(ctx, dir)
				if err != nil {
					return err
				}
				matched := m != nil
				status := "no match"
				if matched {
					status = "MATCH"
					if m.Confidence == detect.ConfidenceWeak {
						status = "MATCH (weak)"
					}
				}
				fmt.Fprintf(out, "%s: %s\n", d.Name(), status)

				if sd, ok := d.(*detect.SpecDetector); ok && sd.Spec != nil {
					trace := sd.Spec.Detect.EvalTrace(ctx, dir)
					fmt.Fprint(out, detect.FormatTrace(trace, "  "))
				} else {
					if matched {
						fmt.Fprintln(out, "  [pass] Go detector")
					} else {
						fmt.Fprintln(out, "  [fail] Go detector")
					}
				}
				if matched && len(m.Targets) > 0 {
					var paths []string
					for _, t := range m.Targets {
						paths = append(paths, t.RelPath)
					}
					fmt.Fprintf(out, "  targets: %s\n", strings.Join(paths, ", "))
				}
				fmt.Fprintln(out)
			}
			return nil
		},
	}
}
