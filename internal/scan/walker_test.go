package scan_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fahid/reclaim/internal/detect"
	_ "github.com/fahid/reclaim/internal/detect/builtin"
	"github.com/fahid/reclaim/internal/plan"
	"github.com/fahid/reclaim/internal/scan"
)

func TestMain(m *testing.M) {
	detect.MustLoadEmbedded()
	os.Exit(m.Run())
}

func TestWalk_PositiveNodeProject(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	nm := filepath.Join(app, "node_modules")
	if err := os.MkdirAll(nm, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "package.json"), []byte(`{"name":"app"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := scan.Walk(scan.Options{Root: root, MaxDepth: 8})
	if err != nil {
		t.Fatal(err)
	}
	p := plan.Build(res, plan.Options{})

	var deletes int
	for _, d := range p.Decisions {
		if d.Verdict == plan.VerdictDelete {
			deletes++
			if d.Target.RelPath != "node_modules" {
				t.Fatalf("unexpected target %q", d.Target.RelPath)
			}
			if d.Project == nil || d.Project.Framework != "nodejs" {
				t.Fatalf("expected nodejs project, got %+v", d.Project)
			}
		}
	}
	if deletes != 1 {
		t.Fatalf("want 1 delete candidate, got %d (decisions=%+v)", deletes, p.Decisions)
	}
}

func TestWalk_OrphanNodeModules(t *testing.T) {
	root := t.TempDir()
	nm := filepath.Join(root, "stray", "node_modules")
	if err := os.MkdirAll(nm, 0o755); err != nil {
		t.Fatal(err)
	}

	res, err := scan.Walk(scan.Options{Root: root, MaxDepth: 8})
	if err != nil {
		t.Fatal(err)
	}
	p := plan.Build(res, plan.Options{})

	if len(p.Decisions) == 0 {
		t.Fatal("expected orphan to appear as skipped")
	}
	for _, d := range p.Decisions {
		if d.Verdict == plan.VerdictDelete {
			t.Fatalf("orphan must not be delete candidate: %+v", d)
		}
		if d.Verdict != plan.VerdictSkipped {
			t.Fatalf("want skipped, got %v", d.Verdict)
		}
	}
}

func TestWalk_CorruptPackageJSON(t *testing.T) {
	root := t.TempDir()
	nm := filepath.Join(root, "node_modules")
	if err := os.MkdirAll(nm, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := scan.Walk(scan.Options{Root: root, MaxDepth: 8})
	if err != nil {
		t.Fatal(err)
	}
	p := plan.Build(res, plan.Options{})

	found := false
	for _, d := range p.Decisions {
		if d.Verdict == plan.VerdictDelete {
			t.Fatalf("weak match must not delete: %+v", d)
		}
		if d.Verdict == plan.VerdictSkipped {
			found = true
		}
	}
	if !found {
		t.Fatal("expected weak/corrupt match to be skipped")
	}
}

func TestWalk_NeverEntersGit(t *testing.T) {
	root := t.TempDir()
	gitNM := filepath.Join(root, ".git", "node_modules")
	if err := os.MkdirAll(gitNM, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "package.json"), []byte(`{"name":"nope"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := scan.Walk(scan.Options{Root: root, MaxDepth: 8})
	if err != nil {
		t.Fatal(err)
	}
	p := plan.Build(res, plan.Options{})
	for _, d := range p.Decisions {
		if d.Verdict == plan.VerdictDelete {
			t.Fatalf("must not target inside .git: %+v", d)
		}
	}
}

func TestWalk_NestedProjects(t *testing.T) {
	root := t.TempDir()
	writeNode := func(dir string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"x"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeNode(root)
	writeNode(filepath.Join(root, "packages", "api"))

	res, err := scan.Walk(scan.Options{Root: root, MaxDepth: 8})
	if err != nil {
		t.Fatal(err)
	}
	p := plan.Build(res, plan.Options{})

	var deletes int
	for _, d := range p.Decisions {
		if d.Verdict == plan.VerdictDelete {
			deletes++
		}
	}
	if deletes != 2 {
		t.Fatalf("want 2 nested delete candidates, got %d", deletes)
	}
}

func TestWalk_SymlinkEscapeSkipped(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	outsideNM := filepath.Join(outside, "leaked")
	if err := os.MkdirAll(outsideNM, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"app"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideNM, filepath.Join(root, "node_modules")); err != nil {
		t.Fatal(err)
	}

	res, err := scan.Walk(scan.Options{Root: root, MaxDepth: 8})
	if err != nil {
		t.Fatal(err)
	}
	p := plan.Build(res, plan.Options{})

	for _, d := range p.Decisions {
		if d.Verdict == plan.VerdictDelete {
			t.Fatalf("symlink escape must not be delete: %+v reason=%s", d, d.Reason)
		}
	}
}
