package plan

import (
	"fmt"
	"io"
	"os"
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

const reasonKeptByControl = "kept by .reclaim.yaml"

// Build turns walk candidates into decisions using safety, control, and git rules.
func Build(res *scan.Result) *Plan {
	p := &Plan{Root: res.Root}
	gitCache := rules.NewGitCache()

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
			p.Decisions = append(p.Decisions, evaluateDeleteCandidate(res.Root, c, gitCache))
		}
	}

	return p
}

func evaluateDeleteCandidate(root string, c scan.Candidate, gitCache *rules.GitCache) Decision {
	base := Decision{Project: c.Project, Target: c.Target}

	if reason := rules.CheckTarget(root, c.Target.Path); reason != "" {
		base.Verdict = VerdictSkipped
		base.Reason = reason
		return base
	}

	isDir := true
	if st, err := os.Lstat(c.Target.Path); err == nil {
		isDir = st.IsDir() || st.Mode()&os.ModeSymlink != 0
	}

	if c.Control != nil {
		keep, del := c.Control.Classify(c.Target.Path, isDir)
		if keep {
			base.Verdict = VerdictKept
			base.Reason = reasonKeptByControl
			return base
		}
		if del != nil {
			c.ExplicitDelete = true
			if del.Reason != "" {
				base.Target.Reason = del.Reason
			}
			if del.Regenerate != "" {
				base.Target.Regenerate = del.Regenerate
			}
		}
	}

	if c.Target.Safety == detect.SafetyRequiresFlag && !c.ExplicitDelete {
		base.Verdict = VerdictSkipped
		base.Reason = "requires --aggressive"
		return base
	}

	bypassGit := c.ExplicitDelete
	if c.Control != nil && !c.Control.RequireGitIgnored() {
		bypassGit = true
	}

	if !bypassGit {
		attachGitRepo(c.Project, gitCache)
		var gitRepo *rules.GitRepo
		if c.Project != nil {
			gitRepo = gitCache.RepoFor(c.Project.Root)
		}
		if reason := rules.CheckGit(gitRepo, c.Target.Path); reason != "" {
			base.Verdict = VerdictSkipped
			base.Reason = reason
			return base
		}
	} else {
		attachGitRepo(c.Project, gitCache)
	}

	base.Verdict = VerdictDelete
	if base.Target.Reason != "" {
		base.Reason = base.Target.Reason
	} else {
		base.Reason = c.Target.Reason
	}
	return base
}

func attachGitRepo(project *detect.Project, cache *rules.GitCache) {
	if project == nil || project.GitRepo != nil {
		return
	}
	r := cache.RepoFor(project.Root)
	if r == nil {
		return
	}
	project.GitRepo = &detect.GitRepo{Root: r.Root}
}

// WriteHuman prints a plan listing to w.
func WriteHuman(w io.Writer, p *Plan) error {
	var deletes, skips, kept []Decision
	for _, d := range p.Decisions {
		switch d.Verdict {
		case VerdictDelete:
			deletes = append(deletes, d)
		case VerdictSkipped:
			skips = append(skips, d)
		case VerdictKept:
			kept = append(kept, d)
		}
	}

	if _, err := fmt.Fprintf(w, "Scanning %s…\n\n", p.Root); err != nil {
		return err
	}

	if len(deletes) == 0 && len(skips) == 0 && len(kept) == 0 {
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

	if len(kept) > 0 {
		if _, err := fmt.Fprintf(w, "Kept (%d)\n", len(kept)); err != nil {
			return err
		}
		for _, d := range kept {
			if _, err := fmt.Fprintf(w, "  %s    %s\n", d.Target.Path, d.Reason); err != nil {
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
				if _, err := fmt.Fprintf(w, "  %s    %s    → %s\n", d.Target.Path, d.Reason, hint); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintf(w, "  %s    %s\n", d.Target.Path, d.Reason); err != nil {
					return err
				}
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	if len(kept) > 0 {
		if _, err := fmt.Fprintf(w, "%d delete · %d kept · %d skipped\n", len(deletes), len(kept), len(skips)); err != nil {
			return err
		}
	} else if _, err := fmt.Fprintf(w, "%d delete · %d skipped\n", len(deletes), len(skips)); err != nil {
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
