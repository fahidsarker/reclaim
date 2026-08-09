package scan

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fahid/reclaim/internal/config"
	"github.com/fahid/reclaim/internal/detect"
	"github.com/fahid/reclaim/internal/rules"
)

// Options controls directory walking.
type Options struct {
	Root             string
	MaxDepth         int
	Concurrency      int
	FollowSymlinks   bool
	CrossDevice      bool     // when false (default), stay on the root's device
	Exclude          []string // doublestar globs relative to Root
	Frameworks       []string // CLI --framework allowlist
	ExcludeFramework []string // CLI --exclude-framework denylist
	IKnowWhatImDoing bool
	NoConfig         bool        // ignore .reclaim.yaml and global config
	Warn             func(string) // optional verbose notes (e.g. skipped symlink cycles)
}

// CandidateKind classifies a walk finding before plan evaluation.
type CandidateKind int

const (
	// KindDeleteCandidate is a Strong-match target that exists on disk.
	KindDeleteCandidate CandidateKind = iota
	// KindOrphan is an artifact with no validated project (e.g. stray node_modules).
	KindOrphan
	// KindWeak is a corrupt/unparseable manifest; never a deletion candidate.
	KindWeak
)

// Candidate is a target discovered during the walk, before safety/git evaluation.
type Candidate struct {
	Kind           CandidateKind
	Project        *detect.Project // nil for orphans
	Target         detect.Target
	Reason         string // set for KindOrphan / KindWeak
	Control        *config.Chain
	ExplicitDelete bool // matched a delete: rule (git / SafetyRequiresFlag bypass)
}

// Result is the outcome of a walk.
type Result struct {
	Root       string
	Candidates []Candidate
	Projects   []*detect.Project
	DirsWalked int
}

// DefaultConcurrency returns min(8, NumCPU).
func DefaultConcurrency() int {
	n := runtime.NumCPU()
	if n > 8 {
		return 8
	}
	if n < 1 {
		return 1
	}
	return n
}

type walkFrame struct {
	dir     string
	depth   int
	parent  *detect.Project
	control *config.Chain
}

// Walk recursively scans root for reclaimable candidates. It never deletes.
func Walk(opts Options) (*Result, error) {
	if opts.MaxDepth < 0 {
		opts.MaxDepth = 8
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = DefaultConcurrency()
	}

	abs, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}
	abs = filepath.Clean(abs)

	if err := rules.GuardRoot(abs, opts.IKnowWhatImDoing); err != nil {
		return nil, err
	}

	info, err := os.Lstat(abs)
	if err != nil {
		return nil, fmt.Errorf("stat root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("scan root is not a directory: %s", abs)
	}
	if isSymlink(info) && !opts.FollowSymlinks {
		return nil, fmt.Errorf("scan root is a symlink (pass --follow-symlinks to allow): %s", abs)
	}

	rootDev, err := fileDevice(abs, info)
	if err != nil {
		rootDev = 0
	}
	visited := map[fileID]struct{}{}
	if opts.FollowSymlinks {
		if id, err := fileIdentity(abs, info); err == nil {
			visited[id] = struct{}{}
		}
	}

	var (
		rootChain    *config.Chain
		globalConfig *config.Global
	)
	if !opts.NoConfig {
		g, err := config.LoadGlobal(abs)
		if err != nil {
			return nil, fmt.Errorf("load global config: %w", err)
		}
		globalConfig = g
		rootChain = &config.Chain{Global: g}
	}

	cache := NewDirCache()
	ctx := &detect.Context{Cache: cache}
	warn := opts.Warn
	if warn == nil {
		warn = func(string) {}
	}

	res := &Result{Root: abs}
	queue := []walkFrame{{dir: abs, depth: 0, parent: nil, control: rootChain}}

	pruneBasenames := map[string]struct{}{}
	for _, b := range detect.PruneBasenames() {
		pruneBasenames[b] = struct{}{}
	}

	for len(queue) > 0 {
		frame := queue[0]
		queue = queue[1:]

		if frame.depth > opts.MaxDepth {
			continue
		}

		base := filepath.Base(frame.dir)
		if rules.IsVCSDir(base) {
			continue
		}

		st, err := cache.Lstat(frame.dir)
		if err != nil {
			continue
		}
		if isSymlink(st) && !opts.FollowSymlinks {
			continue
		}
		if !st.IsDir() && !isSymlink(st) {
			continue
		}
		idInfo := st
		if opts.FollowSymlinks && isSymlink(st) {
			if resolved, err := cache.Stat(frame.dir); err == nil {
				idInfo = resolved
			}
		}
		if opts.FollowSymlinks {
			if id, err := fileIdentity(frame.dir, idInfo); err == nil {
				if _, seen := visited[id]; seen && frame.depth > 0 {
					warn(fmt.Sprintf("skip symlink cycle: %s", frame.dir))
					continue
				}
				visited[id] = struct{}{}
			}
		}
		if !opts.CrossDevice && rootDev != 0 {
			if dev, err := fileDevice(frame.dir, idInfo); err == nil && dev != rootDev {
				warn(fmt.Sprintf("skip cross-device path: %s", frame.dir))
				continue
			}
		}
		if excludedPath(abs, frame.dir, opts.Exclude) {
			warn(fmt.Sprintf("skip excluded path: %s", frame.dir))
			continue
		}

		chain := frame.control
		if !opts.NoConfig {
			local, err := config.LoadControl(frame.dir)
			if err != nil {
				return nil, err
			}
			if local != nil {
				if frame.control != nil {
					chain = frame.control.WithLocal(local)
				} else {
					chain = (&config.Chain{Global: globalConfig}).WithLocal(local)
				}
			}
		}

		if chain != nil && chain.Ignore() {
			continue
		}

		res.DirsWalked++

		var project *detect.Project
		ownedRel := map[string]struct{}{}
		localPrune := map[string]struct{}{}

		mode := config.ModeMerge
		if chain != nil {
			mode = chain.Mode()
		}

		switch mode {
		case config.ModeStrict:
			if _, err := appendExplicitDeletes(res, cache, frame, chain, nil, &ownedRel, &localPrune, pruneBasenames); err != nil {
				return nil, err
			}

		default: // merge
			var only, disable []string
			if chain != nil {
				if eff := chain.Effective(); eff != nil {
					only, disable = eff.FrameworksOnly, eff.FrameworksDisable
				}
			}
			only, disable = mergeFrameworkFilters(opts.Frameworks, opts.ExcludeFramework, only, disable)
			match, err := detect.DetectBestFiltered(ctx, frame.dir, only, disable)
			if err != nil {
				return nil, fmt.Errorf("detect %s: %w", frame.dir, err)
			}

			switch {
			case match != nil && match.Confidence == detect.ConfidenceStrong:
				project = &detect.Project{
					Root:       frame.dir,
					Framework:  match.Framework,
					Confidence: match.Confidence,
					Manifest:   match.Manifest,
					Metadata:   match.Metadata,
					Parent:     frame.parent,
				}
				res.Projects = append(res.Projects, project)

				expanded, err := expandMatchTargets(cache, frame.dir, match.Targets)
				if err != nil {
					return nil, err
				}
				for _, t := range expanded {
					ownedRel[t.RelPath] = struct{}{}
					seg := firstPathSeg(t.RelPath)
					if seg != "" {
						pruneBasenames[seg] = struct{}{}
						localPrune[seg] = struct{}{}
					}
					explicit := false
					if chain != nil {
						if _, del := chain.Classify(t.Path, true); del != nil {
							explicit = true
							if del.Reason != "" && t.Reason == "" {
								t.Reason = del.Reason
							}
						}
					}
					res.Candidates = append(res.Candidates, Candidate{
						Kind:           KindDeleteCandidate,
						Project:        project,
						Target:         t,
						Control:        chain,
						ExplicitDelete: explicit,
					})
				}

			case match != nil && match.Confidence == detect.ConfidenceWeak:
				expanded, err := expandMatchTargets(cache, frame.dir, match.Targets)
				if err != nil {
					return nil, err
				}
				for _, t := range expanded {
					// Explicit delete: upgrades weak paths to delete candidates.
					if chain != nil {
						if _, del := chain.Classify(t.Path, true); del != nil {
							ownedRel[t.RelPath] = struct{}{}
							seg := firstPathSeg(t.RelPath)
							if seg != "" {
								pruneBasenames[seg] = struct{}{}
								localPrune[seg] = struct{}{}
							}
							ct := t
							if del.Reason != "" {
								ct.Reason = del.Reason
							}
							if del.Regenerate != "" {
								ct.Regenerate = del.Regenerate
							}
							res.Candidates = append(res.Candidates, Candidate{
								Kind:           KindDeleteCandidate,
								Project:        syntheticOrParent(frame, chain),
								Target:         ct,
								Control:        chain,
								ExplicitDelete: true,
							})
							continue
						}
					}
					ownedRel[t.RelPath] = struct{}{}
					seg := firstPathSeg(t.RelPath)
					if seg != "" {
						pruneBasenames[seg] = struct{}{}
						localPrune[seg] = struct{}{}
					}
					res.Candidates = append(res.Candidates, Candidate{
						Kind:    KindWeak,
						Target:  t,
						Reason:  "corrupt or unparseable manifest (weak match)",
						Control: chain,
					})
				}
			}

			if _, err := appendExplicitDeletes(res, cache, frame, chain, project, &ownedRel, &localPrune, pruneBasenames); err != nil {
				return nil, err
			}
		}

		entries, err := cache.ReadDir(frame.dir)
		if err != nil {
			continue
		}

		nextParent := frame.parent
		if project != nil {
			nextParent = project
		}
		nextControl := (*config.Chain)(nil)
		if !opts.NoConfig {
			if chain != nil {
				nextControl = chain.ForChildren()
			}
		}

		for _, e := range entries {
			name := e.Name()
			if rules.IsVCSDir(name) {
				continue
			}
			child := filepath.Join(frame.dir, name)
			if excludedPath(abs, child, opts.Exclude) {
				warn(fmt.Sprintf("skip excluded path: %s", child))
				continue
			}

			if _, prune := pruneBasenames[name]; prune {
				if _, local := localPrune[name]; local {
					continue
				}
				if ownedByRel(ownedRel, name) {
					continue
				}
				// Orphan: known artifact without a Strong/Weak owner in this directory.
				// Explicit delete: for this path upgrades to a delete candidate.
				if project == nil {
					if e.IsDir() || (e.Type()&fs.ModeSymlink) != 0 {
						if chain != nil {
							isDir := e.IsDir() || (e.Type()&fs.ModeSymlink) != 0
							if _, del := chain.Classify(child, isDir); del != nil {
								rel := name
								ct := detect.Target{
									Path:       child,
									RelPath:    rel,
									Kind:       detect.KindDir,
									Reason:     del.Reason,
									Regenerate: del.Regenerate,
								}
								res.Candidates = append(res.Candidates, Candidate{
									Kind:           KindDeleteCandidate,
									Project:        syntheticOrParent(frame, chain),
									Target:         ct,
									Control:        chain,
									ExplicitDelete: true,
								})
								localPrune[name] = struct{}{}
								continue
							}
						}
						// In strict mode orphans are irrelevant (detectors off); still skip walking.
						if mode != config.ModeStrict {
							res.Candidates = append(res.Candidates, Candidate{
								Kind: KindOrphan,
								Target: detect.Target{
									Path:    child,
									RelPath: name,
									Kind:    detect.KindDir,
								},
								Reason:  "orphaned: no validated project in the same directory",
								Control: chain,
							})
						}
					}
				}
				continue
			}

			info, err := e.Info()
			if err != nil {
				continue
			}
			if isSymlink(info) {
				if !opts.FollowSymlinks {
					continue
				}
			}
			if !info.IsDir() && !isSymlink(info) {
				continue
			}
			if !info.IsDir() {
				target, err := cache.Stat(child)
				if err != nil || !target.IsDir() {
					continue
				}
			}
			if !opts.CrossDevice && rootDev != 0 {
				childInfo := info
				if isSymlink(info) {
					if st, err := cache.Stat(child); err == nil {
						childInfo = st
					}
				}
				if dev, err := fileDevice(child, childInfo); err == nil && dev != rootDev {
					warn(fmt.Sprintf("skip cross-device path: %s", child))
					continue
				}
			}
			if opts.FollowSymlinks {
				idInfo := info
				if isSymlink(info) {
					if st, err := cache.Stat(child); err == nil {
						idInfo = st
					}
				}
				if id, err := fileIdentity(child, idInfo); err == nil {
					if _, seen := visited[id]; seen {
						warn(fmt.Sprintf("skip symlink cycle: %s", child))
						continue
					}
				}
			}

			queue = append(queue, walkFrame{
				dir:     child,
				depth:   frame.depth + 1,
				parent:  nextParent,
				control: nextControl,
			})
		}
	}

	_ = opts.Concurrency // reserved for parallel detect; walk is sequential for attribution

	if err := detect.RunPostProcessors(res.Projects); err != nil {
		return nil, err
	}
	return res, nil
}

func syntheticOrParent(frame walkFrame, chain *config.Chain) *detect.Project {
	if frame.parent != nil {
		return frame.parent
	}
	dir := frame.dir
	if chain != nil {
		if eff := chain.Effective(); eff != nil {
			dir = eff.Dir
		}
	}
	return &detect.Project{
		Root:       dir,
		Framework:  "custom",
		Confidence: detect.ConfidenceStrong,
	}
}

// appendExplicitDeletes adds delete: matches among direct children of frame.dir
// that are not already owned. Returns whether any were added.
func appendExplicitDeletes(
	res *Result,
	cache *DirCache,
	frame walkFrame,
	chain *config.Chain,
	project *detect.Project,
	ownedRel *map[string]struct{},
	localPrune *map[string]struct{},
	pruneBasenames map[string]struct{},
) (bool, error) {
	if chain == nil {
		return false, nil
	}
	entries, err := cache.ReadDir(frame.dir)
	if err != nil {
		return false, nil
	}
	added := false
	for _, e := range entries {
		name := e.Name()
		if rules.IsVCSDir(name) {
			continue
		}
		if _, ok := (*ownedRel)[name]; ok {
			continue
		}
		child := filepath.Join(frame.dir, name)
		info, err := e.Info()
		if err != nil {
			continue
		}
		isDir := info.IsDir() || isSymlink(info)
		keep, del := chain.Classify(child, isDir)
		if keep || del == nil {
			continue
		}
		// Stay inside the scan root.
		if !IsUnder(child, res.Root) {
			continue
		}
		ct := detect.Target{
			Path:       child,
			RelPath:    name,
			Kind:       detect.KindDir,
			Reason:     del.Reason,
			Regenerate: del.Regenerate,
		}
		if !isDir {
			ct.Kind = detect.KindFile
		}
		owner := project
		if owner == nil {
			owner = syntheticOrParent(frame, chain)
		}
		res.Candidates = append(res.Candidates, Candidate{
			Kind:           KindDeleteCandidate,
			Project:        owner,
			Target:         ct,
			Control:        chain,
			ExplicitDelete: true,
		})
		(*ownedRel)[name] = struct{}{}
		(*localPrune)[name] = struct{}{}
		pruneBasenames[name] = struct{}{}
		added = true
	}
	return added, nil
}

func expandMatchTargets(cache *DirCache, dir string, targets []detect.Target) ([]detect.Target, error) {
	var out []detect.Target
	for _, t := range targets {
		switch t.Kind {
		case detect.KindGlob:
			pattern := filepath.ToSlash(t.RelPath)
			matches, err := doublestar.Glob(os.DirFS(dir), pattern)
			if err != nil {
				continue
			}
			for _, m := range matches {
				rel := filepath.FromSlash(m)
				absTarget := filepath.Join(dir, rel)
				if exists, _ := pathExists(cache, absTarget); !exists {
					continue
				}
				ct := t
				ct.RelPath = rel
				ct.Path = absTarget
				ct.Kind = detect.KindDir
				out = append(out, ct)
			}
		default:
			absTarget := filepath.Join(dir, t.RelPath)
			if exists, _ := pathExists(cache, absTarget); !exists {
				continue
			}
			ct := t
			ct.Path = absTarget
			out = append(out, ct)
		}
	}
	return out, nil
}

func firstPathSeg(rel string) string {
	rel = filepath.ToSlash(rel)
	if rel == "" {
		return ""
	}
	return strings.SplitN(rel, "/", 2)[0]
}

func ownedByRel(owned map[string]struct{}, name string) bool {
	if _, ok := owned[name]; ok {
		return true
	}
	prefix := name + string(filepath.Separator)
	slashPrefix := name + "/"
	for rel := range owned {
		if rel == name {
			return true
		}
		if strings.HasPrefix(rel, prefix) || strings.HasPrefix(filepath.ToSlash(rel), slashPrefix) {
			return true
		}
	}
	return false
}

func isSymlink(info fs.FileInfo) bool {
	return info.Mode()&os.ModeSymlink != 0
}

func pathExists(cache *DirCache, path string) (exists bool, isLink bool) {
	info, err := cache.Lstat(path)
	if err != nil {
		return false, false
	}
	return true, isSymlink(info)
}

// IsUnder reports whether path is equal to or under root.
func IsUnder(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func excludedPath(root, absPath string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return false
	}
	for _, pat := range patterns {
		pat = filepath.ToSlash(strings.TrimSpace(pat))
		if pat == "" {
			continue
		}
		if ok, _ := doublestar.Match(pat, rel); ok {
			return true
		}
		// Also match when pattern is rooted at /
		if strings.HasPrefix(pat, "/") {
			if ok, _ := doublestar.Match(strings.TrimPrefix(pat, "/"), rel); ok {
				return true
			}
		}
	}
	return false
}

func mergeFrameworkFilters(cliOnly, cliExclude, ctrlOnly, ctrlDisable []string) (only, disable []string) {
	only = append([]string(nil), ctrlOnly...)
	if len(cliOnly) > 0 {
		if len(only) == 0 {
			only = append([]string(nil), cliOnly...)
		} else {
			only = intersectStrings(only, cliOnly)
		}
	}
	disable = append(append([]string(nil), ctrlDisable...), cliExclude...)
	return only, disable
}

func intersectStrings(a, b []string) []string {
	set := map[string]struct{}{}
	for _, s := range b {
		set[s] = struct{}{}
	}
	var out []string
	for _, s := range a {
		if _, ok := set[s]; ok {
			out = append(out, s)
		}
	}
	return out
}
