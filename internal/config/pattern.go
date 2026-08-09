package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Matcher is a gitignore-style pattern list matched relative to a base directory.
type Matcher struct {
	base     string // absolute directory patterns are relative to
	patterns []compiledPattern
}

type compiledPattern struct {
	negate  bool
	dirOnly bool
	glob    string // slash-separated doublestar pattern
	raw     string
}

// CompileMatcher compiles gitignore-style patterns relative to baseDir.
// Patterns containing ".." path segments are rejected.
func CompileMatcher(baseDir string, patterns []string) (*Matcher, error) {
	abs, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, err
	}
	abs = filepath.Clean(abs)
	m := &Matcher{base: abs}
	for _, p := range patterns {
		cp, err := compilePattern(p)
		if err != nil {
			return nil, err
		}
		if cp == nil {
			continue
		}
		m.patterns = append(m.patterns, *cp)
	}
	return m, nil
}

func compilePattern(raw string) (*compiledPattern, error) {
	p := strings.TrimSpace(raw)
	if p == "" {
		return nil, nil
	}
	negate := false
	if strings.HasPrefix(p, "!") {
		negate = true
		p = p[1:]
	}
	if p == "" {
		return nil, fmt.Errorf("empty pattern after negation")
	}
	if hasPathTraversal(p) {
		return nil, fmt.Errorf("pattern %q contains path traversal (..)", raw)
	}

	dirOnly := strings.HasSuffix(p, "/")
	if dirOnly {
		p = strings.TrimSuffix(p, "/")
	}
	anchored := strings.HasPrefix(p, "/")
	if anchored {
		p = strings.TrimPrefix(p, "/")
	}
	p = filepath.ToSlash(p)
	if p == "" {
		return nil, fmt.Errorf("invalid pattern %q", raw)
	}

	var glob string
	switch {
	case anchored:
		glob = p
	case strings.Contains(p, "/"):
		// Contains a slash: relative to the control file directory (gitignore rules).
		glob = p
	default:
		// Match at any depth under the control directory.
		glob = "**/" + p
	}

	return &compiledPattern{
		negate:  negate,
		dirOnly: dirOnly,
		glob:    glob,
		raw:     raw,
	}, nil
}

func hasPathTraversal(p string) bool {
	p = filepath.ToSlash(p)
	if strings.HasPrefix(p, "!") {
		p = p[1:]
	}
	p = strings.TrimPrefix(p, "/")
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}

// Match reports whether absPath is selected by the matcher.
// Last matching pattern wins; negation clears a prior match (gitignore semantics).
func (m *Matcher) Match(absPath string, isDir bool) bool {
	if m == nil || len(m.patterns) == 0 {
		return false
	}
	rel, err := filepath.Rel(m.base, filepath.Clean(absPath))
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return false
	}
	if rel == "." {
		return false
	}
	return m.matchRel(rel, isDir)
}

func (m *Matcher) matchRel(rel string, isDir bool) bool {
	matched := false
	for _, p := range m.patterns {
		if p.dirOnly && !isDir {
			continue
		}
		ok, err := doublestar.Match(p.glob, rel)
		if err != nil || !ok {
			// Unanchored basename patterns also try the path as-is when glob is **/name
			// (doublestar usually matches, but be defensive for exact root-level names).
			if !ok && strings.HasPrefix(p.glob, "**/") {
				ok, err = doublestar.Match(strings.TrimPrefix(p.glob, "**/"), rel)
			}
			if err != nil || !ok {
				continue
			}
		}
		if p.negate {
			matched = false
		} else {
			matched = true
		}
	}
	return matched
}

// Base returns the absolute directory patterns are relative to.
func (m *Matcher) Base() string {
	if m == nil {
		return ""
	}
	return m.base
}
