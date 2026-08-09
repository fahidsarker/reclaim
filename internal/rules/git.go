package rules

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/go-git/go-git/v5/plumbing/format/index"
)

// Exact Decision.Reason strings for git vetoes (spec.md §7.3).
const (
	ReasonTracked     = "tracked by git"
	ReasonNotIgnored  = "not in .gitignore"
	ReasonUncommitted = "uncommitted changes"
)

// GitRepo is an opened git worktree used for ignore/tracked/dirty checks.
type GitRepo struct {
	Root    string // absolute worktree root
	repo    *git.Repository
	matcher gitignore.Matcher
	index   *index.Index
}

// GitOptions controls how ignore matching is performed.
type GitOptions struct {
	UseBinary bool // shell out to `git check-ignore` for ignore checks
}

// GitInspection is a detailed git status for explain output.
type GitInspection struct {
	RepoRoot string
	Ignored  bool
	Tracked  bool
	Dirty    bool
}

// GitCache memoises FindGitRepo / open work per worktree root.
type GitCache struct {
	UseBinary bool
	byStart   map[string]*GitRepo // start dir → repo (or nil sentinel via miss)
	byRoot    map[string]*GitRepo
	misses    map[string]struct{}
}

// NewGitCache returns an empty cache.
func NewGitCache() *GitCache {
	return &GitCache{
		byStart: make(map[string]*GitRepo),
		byRoot:  make(map[string]*GitRepo),
		misses:  make(map[string]struct{}),
	}
}

// NewGitCacheWithOptions returns a cache with ignore-backend options.
func NewGitCacheWithOptions(opts GitOptions) *GitCache {
	c := NewGitCache()
	c.UseBinary = opts.UseBinary
	return c
}

// RepoFor walks up from start to find a git repo, caching the result.
func (c *GitCache) RepoFor(start string) *GitRepo {
	if c == nil {
		r, _ := FindGitRepo(start)
		return r
	}
	abs, err := filepath.Abs(start)
	if err != nil {
		return nil
	}
	abs = filepath.Clean(abs)
	if _, miss := c.misses[abs]; miss {
		return nil
	}
	if r, ok := c.byStart[abs]; ok {
		return r
	}
	r, err := FindGitRepo(abs)
	if err != nil || r == nil {
		c.misses[abs] = struct{}{}
		c.byStart[abs] = nil
		return nil
	}
	if existing, ok := c.byRoot[r.Root]; ok {
		c.byStart[abs] = existing
		return existing
	}
	c.byRoot[r.Root] = r
	c.byStart[abs] = r
	return r
}

// FindGitRepo walks up from start looking for a .git directory or gitfile.
// Returns nil if no repository is found.
func FindGitRepo(start string) (*GitRepo, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	dir = filepath.Clean(dir)

	for {
		gitMeta := filepath.Join(dir, ".git")
		if st, err := os.Lstat(gitMeta); err == nil && (st.IsDir() || st.Mode().IsRegular()) {
			return openGitRepo(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, nil
		}
		dir = parent
	}
}

func openGitRepo(root string) (*GitRepo, error) {
	repo, err := git.PlainOpen(root)
	if err != nil {
		return nil, err
	}

	fs := osfs.New(root)
	var patterns []gitignore.Pattern

	rootFS := osfs.New("/")
	if sys, err := gitignore.LoadSystemPatterns(rootFS); err == nil {
		patterns = append(patterns, sys...)
	}
	if global, err := gitignore.LoadGlobalPatterns(rootFS); err == nil {
		patterns = append(patterns, global...)
	}
	local, err := gitignore.ReadPatterns(fs, nil)
	if err != nil {
		return nil, fmt.Errorf("read gitignore patterns: %w", err)
	}
	patterns = append(patterns, local...)

	idx, err := repo.Storer.Index()
	if err != nil {
		// Empty repo without index yet — treat as no tracked files.
		idx = &index.Index{}
	}

	return &GitRepo{
		Root:    root,
		repo:    repo,
		matcher: gitignore.NewMatcher(patterns),
		index:   idx,
	}, nil
}

// CheckGit evaluates git vetoes for targetPath.
// Returns "" if deletion is allowed, otherwise one of the Reason* constants.
// A nil repo means no git repository — deletion is allowed.
func CheckGit(repo *GitRepo, targetPath string) string {
	return CheckGitOpts(repo, targetPath, GitOptions{})
}

// CheckGitOpts is CheckGit with ignore-backend options.
func CheckGitOpts(repo *GitRepo, targetPath string, opts GitOptions) string {
	if repo == nil {
		return ""
	}

	abs, err := filepath.Abs(targetPath)
	if err != nil {
		return ReasonNotIgnored
	}
	abs = filepath.Clean(abs)

	rel, err := filepath.Rel(repo.Root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ReasonNotIgnored
	}
	relSlash := filepath.ToSlash(rel)

	if hasUncommittedInside(repo, relSlash) {
		return ReasonUncommitted
	}
	if isTracked(repo, relSlash) {
		return ReasonTracked
	}
	if isIgnoredOpts(repo, relSlash, abs, opts) {
		return ""
	}
	return ReasonNotIgnored
}

// InspectGit returns detailed git facts for explain without changing CheckGit semantics.
func InspectGit(repo *GitRepo, targetPath string, opts GitOptions) GitInspection {
	if repo == nil {
		return GitInspection{}
	}
	abs, err := filepath.Abs(targetPath)
	if err != nil {
		return GitInspection{RepoRoot: repo.Root}
	}
	abs = filepath.Clean(abs)
	rel, err := filepath.Rel(repo.Root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return GitInspection{RepoRoot: repo.Root}
	}
	relSlash := filepath.ToSlash(rel)
	return GitInspection{
		RepoRoot: repo.Root,
		Ignored:  isIgnoredOpts(repo, relSlash, abs, opts),
		Tracked:  isTracked(repo, relSlash),
		Dirty:    hasUncommittedInside(repo, relSlash),
	}
}

func isTracked(repo *GitRepo, relSlash string) bool {
	if repo.index == nil {
		return false
	}
	prefix := relSlash + "/"
	for _, e := range repo.index.Entries {
		name := e.Name
		if name == relSlash || strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func hasUncommittedInside(repo *GitRepo, relSlash string) bool {
	wt, err := repo.repo.Worktree()
	if err != nil {
		return false
	}
	status, err := wt.Status()
	if err != nil {
		return false
	}
	prefix := relSlash + "/"
	for path, st := range status {
		if path != relSlash && !strings.HasPrefix(path, prefix) {
			continue
		}
		// Untracked entries are handled by ignore / not-in-gitignore, not dirty.
		if st.Staging == git.Untracked && st.Worktree == git.Untracked {
			continue
		}
		if st.Staging == git.Unmodified && st.Worktree == git.Unmodified {
			continue
		}
		// Ignored paths often appear as Untracked; skip those that are ignored.
		if st.Worktree == git.Untracked || st.Staging == git.Untracked {
			if isIgnoredPath(repo, path) {
				continue
			}
		}
		return true
	}
	return false
}

func isIgnored(repo *GitRepo, relSlash, abs string) bool {
	return isIgnoredOpts(repo, relSlash, abs, GitOptions{})
}

func isIgnoredOpts(repo *GitRepo, relSlash, abs string, opts GitOptions) bool {
	if opts.UseBinary {
		if ok, err := gitCheckIgnoreBinary(repo.Root, relSlash); err == nil {
			return ok
		}
		// Fall back to go-git matcher if git is unavailable.
	}
	info, err := os.Lstat(abs)
	isDir := err == nil && info.IsDir()
	return isIgnoredPathDir(repo, relSlash, isDir)
}

func isIgnoredPath(repo *GitRepo, relSlash string) bool {
	abs := filepath.Join(repo.Root, filepath.FromSlash(relSlash))
	info, err := os.Lstat(abs)
	isDir := err == nil && info.IsDir()
	return isIgnoredPathDir(repo, relSlash, isDir)
}

func isIgnoredPathDir(repo *GitRepo, relSlash string, isDir bool) bool {
	if repo.matcher == nil || relSlash == "" || relSlash == "." {
		return false
	}
	parts := strings.Split(relSlash, "/")
	return repo.matcher.Match(parts, isDir)
}

// gitCheckIgnoreBinary runs `git check-ignore --stdin -z` for one relative path.
// Exit 0 means ignored; exit 1 means not ignored.
func gitCheckIgnoreBinary(repoRoot, relSlash string) (bool, error) {
	cmd := exec.Command("git", "-C", repoRoot, "check-ignore", "--stdin", "-z")
	cmd.Stdin = bytes.NewReader([]byte(relSlash + "\x00"))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.Len() > 0, nil
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

// HintForReason returns the Skipped-section hint for a git (or other) reason.
func HintForReason(reason, targetBase string) string {
	switch reason {
	case ReasonNotIgnored:
		if targetBase == "" {
			return "add path to .gitignore"
		}
		return fmt.Sprintf("add `%s` to .gitignore", targetBase)
	case ReasonTracked:
		return "intentional? use `keep:` to silence"
	case ReasonUncommitted:
		return "commit or stash first"
	default:
		return ""
	}
}
