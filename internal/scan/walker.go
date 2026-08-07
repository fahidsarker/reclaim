package scan

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fahid/reclaim/internal/detect"
	"github.com/fahid/reclaim/internal/rules"
)

// Options controls directory walking.
type Options struct {
	Root             string
	MaxDepth         int
	Concurrency      int
	FollowSymlinks   bool
	IKnowWhatImDoing bool
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
	Kind    CandidateKind
	Project *detect.Project // nil for orphans
	Target  detect.Target
	Reason  string // set for KindOrphan / KindWeak
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
	dir    string
	depth  int
	parent *detect.Project
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

	cache := NewDirCache()
	ctx := &detect.Context{Cache: cache}

	res := &Result{Root: abs}
	queue := []walkFrame{{dir: abs, depth: 0, parent: nil}}

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

		res.DirsWalked++

		match, err := detect.DetectBest(ctx, frame.dir)
		if err != nil {
			return nil, fmt.Errorf("detect %s: %w", frame.dir, err)
		}

		var project *detect.Project
		// Paths (relative) owned by this directory's match — do not orphan-report them.
		ownedRel := map[string]struct{}{}
		// First path segments to prune under this directory.
		localPrune := map[string]struct{}{}

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
				res.Candidates = append(res.Candidates, Candidate{
					Kind:    KindDeleteCandidate,
					Project: project,
					Target:  t,
				})
			}

		case match != nil && match.Confidence == detect.ConfidenceWeak:
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
				res.Candidates = append(res.Candidates, Candidate{
					Kind:   KindWeak,
					Target: t,
					Reason: "corrupt or unparseable manifest (weak match)",
				})
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

		for _, e := range entries {
			name := e.Name()
			if rules.IsVCSDir(name) {
				continue
			}
			child := filepath.Join(frame.dir, name)

			if _, prune := pruneBasenames[name]; prune {
				// Owned by current match (exact basename or nested under first segment).
				if _, local := localPrune[name]; local {
					continue
				}
				if ownedByRel(ownedRel, name) {
					continue
				}
				// Orphan: known artifact without a Strong/Weak owner in this directory.
				if project == nil {
					if e.IsDir() || (e.Type()&fs.ModeSymlink) != 0 {
						if match == nil || match.Confidence != detect.ConfidenceWeak {
							res.Candidates = append(res.Candidates, Candidate{
								Kind: KindOrphan,
								Target: detect.Target{
									Path:    child,
									RelPath: name,
									Kind:    detect.KindDir,
								},
								Reason: "orphaned: no validated project in the same directory",
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

			queue = append(queue, walkFrame{
				dir:    child,
				depth:  frame.depth + 1,
				parent: nextParent,
			})
		}
	}

	_ = opts.Concurrency // reserved for parallel detect; walk is sequential for attribution
	return res, nil
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
