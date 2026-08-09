package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fahid/reclaim/internal/config"
	"github.com/fahid/reclaim/internal/detect"
	"github.com/fahid/reclaim/internal/rules"
	"github.com/fahid/reclaim/internal/scan"
)

// Explanation is a full verdict trace for one path (spec.md §10).
type Explanation struct {
	Path       string
	Project    *detect.Project
	Detector   string
	TargetRel  string
	Regenerate string
	Target     *detect.Target
	Git        rules.GitInspection
	Control    string
	ControlHit string
	Verdict    Verdict
	Reason     string
	Blocking   string
}

// ExplainOptions controls explain evaluation.
type ExplainOptions struct {
	Aggressive   bool
	UseGitBinary bool
	NoConfig     bool
}

// ExplainPath explains why path is or isn't a reclaim candidate.
func ExplainPath(path string, opts ExplainOptions) (*Explanation, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	abs = filepath.Clean(abs)

	st, err := os.Lstat(abs)
	if err != nil {
		return nil, err
	}
	isDir := st.IsDir() || st.Mode()&os.ModeSymlink != 0

	cache := scan.NewDirCache()
	ctx := &detect.Context{Cache: cache}

	projectDir := abs
	if !st.IsDir() {
		projectDir = filepath.Dir(abs)
	}

	var (
		match   *detect.Match
		projDir string
	)
	for dir := projectDir; ; {
		m, err := detect.DetectBest(ctx, dir)
		if err != nil {
			return nil, err
		}
		if m != nil && m.Confidence == detect.ConfidenceStrong {
			// Prefer a match that owns this path as a target.
			if ownsPath(dir, m, abs) || match == nil {
				match = m
				projDir = dir
				if ownsPath(dir, m, abs) {
					break
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	ex := &Explanation{Path: abs}
	gitOpts := rules.GitOptions{UseBinary: opts.UseGitBinary}

	var chain *config.Chain
	if !opts.NoConfig {
		chain, err = nearestChain(projectDir)
		if err != nil {
			return nil, err
		}
	}

	if match != nil {
		ex.Project = &detect.Project{
			Root:       projDir,
			Framework:  match.Framework,
			Confidence: match.Confidence,
			Manifest:   match.Manifest,
			Metadata:   match.Metadata,
		}
		ex.Detector = match.Framework
		rel, _ := filepath.Rel(projDir, abs)
		ex.TargetRel = rel
		for _, t := range match.Targets {
			tp := filepath.Clean(filepath.Join(projDir, t.RelPath))
			if tp == abs {
				tt := t
				tt.Path = abs
				ex.Target = &tt
				ex.TargetRel = t.RelPath
				ex.Regenerate = t.Regenerate
				break
			}
		}
	}

	ex.Control, ex.ControlHit = describeControl(chain, abs, isDir)

	cand := scan.Candidate{
		Kind:    scan.KindDeleteCandidate,
		Project: ex.Project,
		Control: chain,
	}
	if ex.Target != nil {
		cand.Target = *ex.Target
	} else {
		cand.Target = detect.Target{
			Path:    abs,
			RelPath: ex.TargetRel,
			Kind:    detect.KindDir,
		}
		if !isDir {
			cand.Target.Kind = detect.KindFile
		}
	}

	if ex.Target == nil && match == nil {
		// Orphan artifact?
		base := filepath.Base(abs)
		for _, b := range detect.PruneBasenames() {
			if b == base {
				ex.Verdict = VerdictSkipped
				ex.Reason = "orphaned: no validated project in the same directory"
				fillGitExplain(ex, abs, gitOpts)
				return ex, nil
			}
		}
		ex.Verdict = VerdictSkipped
		ex.Reason = "not a reclaim candidate"
		fillGitExplain(ex, abs, gitOpts)
		return ex, nil
	}

	if ex.Target == nil {
		// Path is inside a project but not a listed target.
		ex.Verdict = VerdictSkipped
		ex.Reason = "not a reclaim candidate"
		fillGitExplain(ex, abs, gitOpts)
		return ex, nil
	}

	exists, _ := pathExists(abs)
	if !exists {
		ex.Verdict = VerdictSkipped
		ex.Reason = "path does not exist"
		fillGitExplain(ex, abs, gitOpts)
		return ex, nil
	}

	gitCache := rules.NewGitCacheWithOptions(gitOpts)
	d := evaluateDeleteCandidate(projDir, cand, gitCache, Options{
		Aggressive:   opts.Aggressive,
		UseGitBinary: opts.UseGitBinary,
	})
	ex.Verdict = d.Verdict
	ex.Reason = d.Reason
	if d.Target.Regenerate != "" {
		ex.Regenerate = d.Target.Regenerate
	}
	fillGitExplain(ex, abs, gitOpts)
	return ex, nil
}

func ownsPath(projectDir string, m *detect.Match, abs string) bool {
	for _, t := range m.Targets {
		if filepath.Clean(filepath.Join(projectDir, t.RelPath)) == abs {
			return true
		}
	}
	return false
}

func pathExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func nearestChain(start string) (*config.Chain, error) {
	dir := start
	var nearest *config.Control
	var ancestors []*config.Control
	for {
		c, err := config.LoadControl(dir)
		if err != nil {
			return nil, err
		}
		if c != nil {
			if nearest == nil {
				nearest = c
			} else {
				ancestors = append(ancestors, c)
			}
			if !c.Inherit {
				break
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if nearest == nil && len(ancestors) == 0 {
		return nil, nil
	}
	return &config.Chain{Nearest: nearest, Ancestors: ancestors}, nil
}

func describeControl(chain *config.Chain, abs string, isDir bool) (desc, hit string) {
	if chain == nil || chain.Effective() == nil {
		return "none", ""
	}
	eff := chain.Effective()
	path := filepath.Join(eff.Dir, ".reclaim.yaml")
	inherited := chain.Nearest == nil || (chain.Nearest != nil && chain.Nearest.Dir != filepath.Dir(abs) && len(chain.Ancestors) > 0)
	mode := eff.Mode.String()
	if inherited || (chain.Nearest != nil && chain.Nearest.Dir != filepath.Clean(filepath.Dir(abs))) {
		desc = fmt.Sprintf("%s (inherited, mode=%s)", path, mode)
	} else {
		desc = fmt.Sprintf("%s (mode=%s)", path, mode)
	}
	keep, del := chain.Classify(abs, isDir)
	switch {
	case keep:
		hit = "keep: matched"
	case del != nil:
		hit = "delete: matched"
	default:
		hit = "no matching rule"
	}
	return desc, hit
}

func fillGitExplain(ex *Explanation, abs string, opts rules.GitOptions) {
	start := abs
	if ex.Project != nil {
		start = ex.Project.Root
	}
	repo, _ := rules.FindGitRepo(start)
	ex.Git = rules.InspectGit(repo, abs, opts)
	switch {
	case ex.Reason == rules.ReasonUncommitted:
		ex.Blocking = "dirty"
	case ex.Reason == rules.ReasonTracked:
		ex.Blocking = "tracked"
	case ex.Reason == rules.ReasonNotIgnored:
		ex.Blocking = "ignored"
	}
}

// FormatExplanation renders the §10 explain block.
func FormatExplanation(ex *Explanation) string {
	if ex == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", ex.Path)

	if ex.Project != nil {
		conf := "strong"
		if ex.Project.Confidence == detect.ConfidenceWeak {
			conf = "weak"
		}
		manifest := filepath.Base(ex.Project.Manifest)
		if manifest == "." || manifest == "" {
			manifest = "manifest"
		}
		fmt.Fprintf(&b, "  Project:    %s (%s, %s — %s)\n",
			ex.Project.Root, ex.Project.Framework, conf, manifest)
	} else {
		fmt.Fprintln(&b, "  Project:    (none)")
	}

	det := ex.Detector
	if det == "" {
		det = "(none)"
	}
	target := ex.TargetRel
	if target == "" {
		target = filepath.Base(ex.Path)
	}
	fmt.Fprintf(&b, "  Detector:   %s → target `%s`\n", det, target)

	regen := ex.Regenerate
	if regen == "" {
		regen = "(none)"
	}
	fmt.Fprintf(&b, "  Regenerate: %s\n", regen)

	gitRoot := ex.Git.RepoRoot
	if gitRoot == "" {
		gitRoot = "(none)"
	}
	fmt.Fprintf(&b, "  Git repo:   %s\n", gitRoot)

	ignored := yn(ex.Git.Ignored)
	tracked := yn(ex.Git.Tracked)
	if ex.Blocking == "ignored" {
		ignored += "  ← blocking"
	}
	if ex.Blocking == "tracked" {
		tracked += "  ← blocking"
	}
	fmt.Fprintf(&b, "  Ignored:    %s\n", ignored)
	fmt.Fprintf(&b, "  Tracked:    %s\n", tracked)

	ctrl := ex.Control
	if ctrl == "" {
		ctrl = "none"
	}
	if ex.ControlHit != "" {
		ctrl = fmt.Sprintf("%s — %s", ctrl, ex.ControlHit)
	}
	fmt.Fprintf(&b, "  Control:    %s\n", ctrl)
	fmt.Fprintf(&b, "\n  Verdict: %s — %s\n", strings.ToUpper(ex.Verdict.String()), ex.Reason)
	return b.String()
}

func yn(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
