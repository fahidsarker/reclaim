package config

import (
	"path/filepath"
	"testing"
)

func TestParseControl_DefaultsAndDeleteForms(t *testing.T) {
	dir := t.TempDir()
	raw := []byte(`
version: 1
keep:
  - vendor/
delete:
  - tmp/
  - path: build/
    reason: "CMake output"
    regenerate: "cmake --build build"
frameworks:
  only: [nodejs]
  disable: [python]
`)
	c, err := ParseControl(dir, raw)
	if err != nil {
		t.Fatal(err)
	}
	if c.Mode != ModeMerge {
		t.Fatalf("mode=%v", c.Mode)
	}
	if !c.Inherit || !c.RequireGitIgnored {
		t.Fatalf("defaults: inherit=%v require_git=%v", c.Inherit, c.RequireGitIgnored)
	}
	if len(c.Delete) != 2 {
		t.Fatalf("delete len=%d", len(c.Delete))
	}
	if c.Delete[0].Reason != "listed in .reclaim.yaml" {
		t.Fatalf("shorthand reason: %q", c.Delete[0].Reason)
	}
	if c.Delete[1].Reason != "CMake output" || c.Delete[1].Regenerate == "" {
		t.Fatalf("object delete: %+v", c.Delete[1])
	}
	if len(c.FrameworksOnly) != 1 || c.FrameworksOnly[0] != "nodejs" {
		t.Fatalf("frameworks only: %v", c.FrameworksOnly)
	}
}

func TestParseControl_StrictAndRequireGit(t *testing.T) {
	dir := t.TempDir()
	c, err := ParseControl(dir, []byte(`
mode: strict
inherit: false
require_git_ignored: false
ignore: true
delete:
  - custom_cache
`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Mode != ModeStrict || c.Inherit || c.RequireGitIgnored || !c.Ignore {
		t.Fatalf("got %+v", c)
	}
}

func TestParseControl_RejectsBadVersionModeTraversal(t *testing.T) {
	dir := t.TempDir()
	if _, err := ParseControl(dir, []byte(`version: 2`)); err == nil {
		t.Fatal("expected version error")
	}
	if _, err := ParseControl(dir, []byte(`mode: weird`)); err == nil {
		t.Fatal("expected mode error")
	}
	if _, err := ParseControl(dir, []byte("keep:\n  - ../escape\n")); err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestChain_ClassifyPrecedence(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "pkg")

	parent, err := ParseControl(root, []byte(`
keep:
  - node_modules
delete:
  - dist
`))
	if err != nil {
		t.Fatal(err)
	}
	local, err := ParseControl(child, []byte(`
delete:
  - node_modules
`))
	if err != nil {
		t.Fatal(err)
	}

	// Nearest delete beats inherited keep.
	ch := (&Chain{}).WithLocal(parent).WithLocal(local)
	nm := filepath.Join(child, "node_modules")
	keep, del := ch.Classify(nm, true)
	if keep || del == nil {
		t.Fatalf("nearest delete should win: keep=%v del=%v", keep, del)
	}

	// Nearest keep beats nearest delete (separate controls).
	localKeep, err := ParseControl(child, []byte("keep:\n  - dist\n"))
	if err != nil {
		t.Fatal(err)
	}
	ch2 := (&Chain{}).WithLocal(parent).WithLocal(localKeep)
	dist := filepath.Join(child, "dist")
	keep, del = ch2.Classify(dist, true)
	if !keep || del != nil {
		t.Fatalf("nearest keep should win: keep=%v del=%v", keep, del)
	}
}

func TestLoadControl_Missing(t *testing.T) {
	c, err := LoadControl(t.TempDir())
	if err != nil || c != nil {
		t.Fatalf("got c=%v err=%v", c, err)
	}
}

func FuzzParseControl(f *testing.F) {
	f.Add([]byte("version: 1\nmode: merge\n"))
	f.Add([]byte("keep:\n  - node_modules\n"))
	f.Add([]byte("delete:\n  - path: build/\n    reason: x\n"))
	f.Add([]byte("mode: strict\nrequire_git_ignored: false\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		_, _ = ParseControl(dir, data)
	})
}
