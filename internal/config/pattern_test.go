package config

import (
	"path/filepath"
	"testing"
)

func TestCompileMatcher_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{"../outside", "foo/../../etc", "!../x"} {
		if _, err := CompileMatcher(dir, []string{p}); err == nil {
			t.Fatalf("expected error for pattern %q", p)
		}
	}
}

func TestMatcher_GitignoreSemantics(t *testing.T) {
	base := t.TempDir()
	m, err := CompileMatcher(base, []string{
		"node_modules",
		"/onlyroot",
		"build/",
		"*.log",
		"!keep.log",
	})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		rel  string
		dir  bool
		want bool
	}{
		{"node_modules", true, true},
		{"pkg/node_modules", true, true},
		{"onlyroot", true, true},
		{"nested/onlyroot", true, false},
		{"build", true, true},
		{"build", false, false}, // trailing slash → dirs only
		{"deep/build", true, true},
		{"error.log", false, true},
		{"keep.log", false, false}, // negated
		{"src/error.log", false, true},
	}
	for _, tc := range cases {
		abs := filepath.Join(base, filepath.FromSlash(tc.rel))
		if got := m.Match(abs, tc.dir); got != tc.want {
			t.Errorf("Match(%q, dir=%v)=%v want %v", tc.rel, tc.dir, got, tc.want)
		}
	}
}

func TestMatcher_Doublestar(t *testing.T) {
	base := t.TempDir()
	m, err := CompileMatcher(base, []string{"**/.cache"})
	if err != nil {
		t.Fatal(err)
	}
	if !m.Match(filepath.Join(base, ".cache"), true) {
		t.Fatal("expected .cache match")
	}
	if !m.Match(filepath.Join(base, "a", "b", ".cache"), true) {
		t.Fatal("expected nested .cache match")
	}
}

func FuzzMatchPattern(f *testing.F) {
	f.Add("node_modules")
	f.Add("/build/")
	f.Add("**/*.log")
	f.Add("!vendor")
	f.Add("foo/bar")
	f.Fuzz(func(t *testing.T, pattern string) {
		dir := t.TempDir()
		m, err := CompileMatcher(dir, []string{pattern})
		if err != nil {
			// Traversal and invalid patterns are acceptable failures.
			return
		}
		_ = m.Match(filepath.Join(dir, "node_modules"), true)
		_ = m.Match(filepath.Join(dir, "a", "b"), false)
	})
}
