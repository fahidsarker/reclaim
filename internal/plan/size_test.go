package plan_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fahid/reclaim/internal/detect"
	"github.com/fahid/reclaim/internal/plan"
)

func TestSize_SumsNestedFiles(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "node_modules")
	if err := os.MkdirAll(filepath.Join(target, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := []byte("hello")                                // 5
	b := []byte("world!!")                              // 7
	if err := os.WriteFile(filepath.Join(target, "a.txt"), a, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "pkg", "b.txt"), b, 0o644); err != nil {
		t.Fatal(err)
	}

	p := &plan.Plan{
		Root: root,
		Decisions: []plan.Decision{{
			Project: &detect.Project{Root: root, Framework: "nodejs"},
			Target:  detect.Target{Path: target, RelPath: "node_modules", Regenerate: "npm install"},
			Verdict: plan.VerdictDelete,
		}},
	}
	if err := plan.Size(p, plan.SizeOptions{Concurrency: 2}); err != nil {
		t.Fatal(err)
	}
	got := p.Decisions[0].Target.Size
	want := int64(len(a) + len(b))
	if got != want {
		t.Fatalf("size: got %d want %d", got, want)
	}
	if p.Decisions[0].Target.ModTime.IsZero() {
		t.Fatal("expected ModTime to be set")
	}
}

func TestSize_MissingPathUnknown(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "gone")
	p := &plan.Plan{
		Root: root,
		Decisions: []plan.Decision{{
			Target:  detect.Target{Path: missing, RelPath: "gone"},
			Verdict: plan.VerdictDelete,
			Reason:  "test",
		}},
	}
	if err := plan.Size(p, plan.SizeOptions{Concurrency: 1}); err != nil {
		t.Fatal(err)
	}
	if p.Decisions[0].Target.Size != plan.SizeUnknown {
		t.Fatalf("want SizeUnknown, got %d", p.Decisions[0].Target.Size)
	}
	if p.Decisions[0].Verdict != plan.VerdictDelete {
		t.Fatalf("verdict must stay delete, got %v", p.Decisions[0].Verdict)
	}
}

func TestSize_NoSize(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "node_modules")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &plan.Plan{
		Root: root,
		Decisions: []plan.Decision{{
			Target:  detect.Target{Path: target},
			Verdict: plan.VerdictDelete,
		}},
	}
	if err := plan.Size(p, plan.SizeOptions{NoSize: true, Concurrency: 1}); err != nil {
		t.Fatal(err)
	}
	if p.Decisions[0].Target.Size != plan.SizeSkipped {
		t.Fatalf("want SizeSkipped (-1), got %d", p.Decisions[0].Target.Size)
	}
	if p.Decisions[0].Target.ModTime.IsZero() {
		t.Fatal("expected ModTime even with --no-size")
	}
}

func TestSize_SkipsNonDelete(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "node_modules")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "x"), []byte("abcd"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &plan.Plan{
		Root: root,
		Decisions: []plan.Decision{{
			Target:  detect.Target{Path: target},
			Verdict: plan.VerdictSkipped,
			Reason:  "tracked by git",
		}},
	}
	if err := plan.Size(p, plan.SizeOptions{Concurrency: 1}); err != nil {
		t.Fatal(err)
	}
	if p.Decisions[0].Target.Size != 0 {
		t.Fatalf("skipped targets should not be sized, got %d", p.Decisions[0].Target.Size)
	}
}
