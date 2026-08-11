package ui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"golang.org/x/term"

	"github.com/fahid/reclaim/internal/detect"
	"github.com/fahid/reclaim/internal/plan"
	"github.com/fahid/reclaim/internal/rules"
)

// RenderOptions controls human plan output.
type RenderOptions struct {
	NoSize  bool
	NoColor bool
	Quiet   bool  // suppress progress headers when set by caller
	Color   *bool // optional override; nil → auto from NoColor / NO_COLOR / TTY
}

type styles struct {
	header  lipgloss.Style
	project lipgloss.Style
	muted   lipgloss.Style
	warn    lipgloss.Style
	bold    lipgloss.Style
}

func newStyles(color bool) styles {
	if !color {
		return styles{
			header:  lipgloss.NewStyle(),
			project: lipgloss.NewStyle(),
			muted:   lipgloss.NewStyle(),
			warn:    lipgloss.NewStyle(),
			bold:    lipgloss.NewStyle(),
		}
	}
	return styles{
		header:  lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		project: lipgloss.NewStyle().Bold(true),
		muted:   lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		warn:    lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		bold:    lipgloss.NewStyle().Bold(true),
	}
}

func useColor(opts RenderOptions, w io.Writer) bool {
	if opts.Color != nil {
		return *opts.Color
	}
	if opts.NoColor {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

type projectGroup struct {
	root      string
	framework string
	meta      string
	decisions []plan.Decision
	bytes     int64
	unknown   bool
}

// Render writes §10-shaped human plan output to w.
func Render(w io.Writer, p *plan.Plan, opts RenderOptions) error {
	if p == nil {
		return nil
	}
	st := newStyles(useColor(opts, w))

	var deletes, skips, kept []plan.Decision
	for _, d := range p.Decisions {
		switch d.Verdict {
		case plan.VerdictDelete:
			deletes = append(deletes, d)
		case plan.VerdictSkipped:
			skips = append(skips, d)
		case plan.VerdictKept:
			kept = append(kept, d)
		}
	}

	if !opts.Quiet {
		scanLine := fmt.Sprintf("Scanning %s (depth %d)… %s dirs, %s projects, %s",
			p.Root,
			p.Stats.Depth,
			humanize.Comma(int64(p.Stats.DirsWalked)),
			humanize.Comma(int64(p.Stats.Projects)),
			formatDur(p.Stats.ScanDuration),
		)
		if _, err := fmt.Fprintln(w, st.header.Render(scanLine)); err != nil {
			return err
		}
		if !opts.NoSize {
			sizeLine := fmt.Sprintf("Sizing %d targets… done, %s", len(deletes), formatDur(p.Stats.SizeDuration))
			if _, err := fmt.Fprintln(w, st.header.Render(sizeLine)); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	if len(deletes) == 0 && len(skips) == 0 && len(kept) == 0 {
		_, err := fmt.Fprintln(w, "No reclaimable targets found.")
		return err
	}

	groups := groupDeletes(deletes, opts.NoSize)
	for _, g := range groups {
		if err := writeProjectGroup(w, g, opts.NoSize, st); err != nil {
			return err
		}
	}

	if len(kept) > 0 {
		if _, err := fmt.Fprintf(w, "Kept (%d)\n", len(kept)); err != nil {
			return err
		}
		for _, d := range kept {
			if _, err := fmt.Fprintf(w, "  %-40s  %s\n", d.Target.Path, d.Reason); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	if len(skips) > 0 {
		if _, err := fmt.Fprintf(w, "Skipped (%d)\n", len(skips)); err != nil {
			return err
		}
		for _, d := range skips {
			hint := rules.HintForReason(d.Reason, filepath.Base(d.Target.Path))
			if hint != "" {
				if _, err := fmt.Fprintf(w, "  %-40s  %-22s  → %s\n", d.Target.Path, d.Reason, hint); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintf(w, "  %-40s  %s\n", d.Target.Path, d.Reason); err != nil {
					return err
				}
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	if err := writeTotals(w, groups, len(deletes), opts.NoSize, st); err != nil {
		return err
	}

	warn := "Deletion is permanent. Use --to-trash to recover later."
	_, err := fmt.Fprintln(w, st.warn.Render(warn))
	return err
}

func groupDeletes(deletes []plan.Decision, noSize bool) []projectGroup {
	byRoot := map[string]*projectGroup{}
	var order []string
	for _, d := range deletes {
		root := d.Target.Path
		fw := "unknown"
		meta := ""
		if d.Project != nil {
			root = d.Project.Root
			fw = d.Project.Framework
			meta = formatMeta(d.Project)
		}
		g, ok := byRoot[root]
		if !ok {
			g = &projectGroup{root: root, framework: fw, meta: meta}
			byRoot[root] = g
			order = append(order, root)
		}
		g.decisions = append(g.decisions, d)
		switch {
		case d.Target.Size == plan.SizeUnknown:
			g.unknown = true
		case d.Target.Size >= 0:
			g.bytes += d.Target.Size
		}
	}

	groups := make([]projectGroup, 0, len(order))
	for _, root := range order {
		groups = append(groups, *byRoot[root])
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if noSize {
			return groups[i].root < groups[j].root
		}
		if groups[i].bytes != groups[j].bytes {
			return groups[i].bytes > groups[j].bytes
		}
		return groups[i].root < groups[j].root
	})
	for gi := range groups {
		sort.SliceStable(groups[gi].decisions, func(i, j int) bool {
			a, b := groups[gi].decisions[i], groups[gi].decisions[j]
			if !noSize && a.Target.Size != b.Target.Size {
				return a.Target.Size > b.Target.Size
			}
			return plan.DisplayPath(a.Target.Path, a.Project) < plan.DisplayPath(b.Target.Path, b.Project)
		})
	}
	return groups
}

func formatMeta(p *detect.Project) string {
	if p == nil || p.Metadata == nil {
		return ""
	}
	if pm, ok := p.Metadata["packageManager"]; ok && pm != "" {
		return pm
	}
	return ""
}

func writeProjectGroup(w io.Writer, g projectGroup, noSize bool, st styles) error {
	label := g.framework
	if g.meta != "" {
		label = g.framework + " · " + g.meta
	}
	header := fmt.Sprintf("%-56s  %s", g.root, label)
	if _, err := fmt.Fprintln(w, st.project.Render(header)); err != nil {
		return err
	}
	for _, d := range g.decisions {
		rel := plan.DisplayPath(d.Target.Path, d.Project)
		regen := d.Target.Regenerate
		if regen == "" {
			regen = d.Reason
		}
		if noSize {
			if _, err := fmt.Fprintf(w, "  %-30s  %s\n", rel, regen); err != nil {
				return err
			}
			continue
		}
		sizeStr := formatSize(d.Target.Size)
		ageStr := formatAge(d.Target.ModTime)
		if _, err := fmt.Fprintf(w, "  %-30s  %8s  %5s  %s\n", rel, sizeStr, ageStr, regen); err != nil {
			return err
		}
	}
	if !noSize {
		sub := formatSize(g.bytes)
		if g.unknown && g.bytes == 0 {
			sub = "unknown"
		}
		if _, err := fmt.Fprintf(w, "  %-30s  %8s\n", "", "─────────"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  %-30s  %8s\n", "", sub); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

func writeTotals(w io.Writer, groups []projectGroup, deleteCount int, noSize bool, st styles) error {
	if _, err := fmt.Fprintln(w, st.muted.Render(strings.Repeat("─", 57))); err != nil {
		return err
	}
	projects := len(groups)
	if noSize {
		line := fmt.Sprintf(" %d projects · %d targets", projects, deleteCount)
		_, err := fmt.Fprintln(w, st.bold.Render(line))
		return err
	}
	var total int64
	unknown := false
	for _, g := range groups {
		total += g.bytes
		if g.unknown {
			unknown = true
		}
	}
	bytesStr := humanize.Bytes(uint64(total))
	if unknown && total == 0 {
		bytesStr = "unknown"
	}
	line := fmt.Sprintf(" %d projects · %d targets · %s reclaimable", projects, deleteCount, bytesStr)
	_, err := fmt.Fprintln(w, st.bold.Render(line))
	return err
}

func formatSize(n int64) string {
	switch {
	case n == plan.SizeSkipped:
		return ""
	case n == plan.SizeUnknown:
		return "unknown"
	case n < 0:
		return "unknown"
	default:
		return humanize.Bytes(uint64(n))
	}
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	days := int(d.Hours() / 24)
	if days < 1 {
		hours := int(d.Hours())
		if hours < 1 {
			return "<1h"
		}
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dd", days)
}

func formatDur(d time.Duration) string {
	if d <= 0 {
		return "0.0s"
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
