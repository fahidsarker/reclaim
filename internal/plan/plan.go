package plan

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/fahid/reclaim/internal/detect"
	"github.com/fahid/reclaim/internal/rules"
	"github.com/fahid/reclaim/internal/scan"
)

// Verdict is the final decision for a target.
type Verdict int

const (
	VerdictDelete Verdict = iota
	VerdictSkipped
	VerdictKept
)

func (v Verdict) String() string {
	switch v {
	case VerdictDelete:
		return "delete"
	case VerdictSkipped:
		return "skipped"
	case VerdictKept:
		return "kept"
	default:
		return "unknown"
	}
}

// Decision is one evaluated target with a reason.
type Decision struct {
	Project *detect.Project
	Target  detect.Target
	Verdict Verdict
	Reason  string
}

// Plan is the full set of decisions for a scan root.
type Plan struct {
	Root      string
	Decisions []Decision
}

// Build turns walk candidates into decisions using safety rules (Phase 1: no git).
func Build(res *scan.Result) *Plan {
	p := &Plan{Root: res.Root}

	for _, c := range res.Candidates {
		switch c.Kind {
		case scan.KindOrphan:
			p.Decisions = append(p.Decisions, Decision{
				Project: c.Project,
				Target:  c.Target,
				Verdict: VerdictSkipped,
				Reason:  c.Reason,
			})
		case scan.KindWeak:
			p.Decisions = append(p.Decisions, Decision{
				Project: c.Project,
				Target:  c.Target,
				Verdict: VerdictSkipped,
				Reason:  c.Reason,
			})
		case scan.KindDeleteCandidate:
			if reason := rules.CheckTarget(res.Root, c.Target.Path); reason != "" {
				p.Decisions = append(p.Decisions, Decision{
					Project: c.Project,
					Target:  c.Target,
					Verdict: VerdictSkipped,
					Reason:  reason,
				})
				continue
			}
			p.Decisions = append(p.Decisions, Decision{
				Project: c.Project,
				Target:  c.Target,
				Verdict: VerdictDelete,
				Reason:  c.Target.Reason,
			})
		}
	}

	return p
}

// WriteHuman prints a simple Phase-1 plan listing to w.
func WriteHuman(w io.Writer, p *Plan) error {
	var deletes, skips []Decision
	for _, d := range p.Decisions {
		switch d.Verdict {
		case VerdictDelete:
			deletes = append(deletes, d)
		case VerdictSkipped:
			skips = append(skips, d)
		}
	}

	if _, err := fmt.Fprintf(w, "Scanning %s…\n\n", p.Root); err != nil {
		return err
	}

	if len(deletes) == 0 && len(skips) == 0 {
		if _, err := fmt.Fprintln(w, "No reclaimable targets found."); err != nil {
			return err
		}
		return nil
	}

	if len(deletes) > 0 {
		if _, err := fmt.Fprintln(w, "Delete candidates"); err != nil {
			return err
		}
		for _, d := range deletes {
			fw := "unknown"
			proj := d.Target.Path
			if d.Project != nil {
				fw = d.Project.Framework
				proj = d.Project.Root
			}
			extra := ""
			if d.Target.Regenerate != "" {
				extra = "  → " + d.Target.Regenerate
			}
			if _, err := fmt.Fprintf(w, "  %s\n    %s  [%s]  %s%s\n",
				proj,
				displayPath(d.Target.Path, d.Project),
				fw,
				d.Reason,
				extra,
			); err != nil {
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
			if _, err := fmt.Fprintf(w, "  %s\n    %s\n", d.Target.Path, d.Reason); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(w, "%d delete · %d skipped\n", len(deletes), len(skips)); err != nil {
		return err
	}
	return nil
}

func displayPath(abs string, project *detect.Project) string {
	if project == nil {
		return abs
	}
	rel, err := filepath.Rel(project.Root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return abs
	}
	return rel
}
