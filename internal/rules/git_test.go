package rules_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fahid/reclaim/internal/detect"
	"github.com/fahid/reclaim/internal/plan"
	"github.com/fahid/reclaim/internal/rules"
	"github.com/fahid/reclaim/internal/scan"
	"github.com/fahid/reclaim/internal/ui"
	"github.com/fahid/reclaim/testdata/fixtures"
)

func TestMain(m *testing.M) {
	detect.MustLoadEmbedded()
	os.Exit(m.Run())
}

func TestCheckGit_NoRepo(t *testing.T) {
	dir := t.TempDir()
	nm := filepath.Join(dir, "node_modules")
	if err := os.MkdirAll(nm, 0o755); err != nil {
		t.Fatal(err)
	}
	if reason := rules.CheckGit(nil, nm); reason != "" {
		t.Fatalf("nil repo should allow delete, got %q", reason)
	}
	repo, err := rules.FindGitRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	if repo != nil {
		t.Fatalf("expected no repo, got %+v", repo)
	}
}

func TestGit_IgnoredNodeModules_Delete(t *testing.T) {
	root := t.TempDir()
	if err := fixtures.NodeJS(root); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.GitInit(root, "package.json", ".gitignore"); err != nil {
		t.Fatal(err)
	}

	p := buildPlan(t, root)
	d := mustDecision(t, p, "node_modules")
	if d.Verdict != plan.VerdictDelete {
		t.Fatalf("ignored node_modules want delete, got %v (%s)", d.Verdict, d.Reason)
	}
	if d.Project == nil || d.Project.GitRepo == nil {
		t.Fatal("expected GitRepo attached to project")
	}
}

func TestGit_NotIgnored_Skipped(t *testing.T) {
	root := t.TempDir()
	if err := fixtures.WriteFile(filepath.Join(root, "package.json"), `{"name":"app"}`); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.Mkdir(filepath.Join(root, "node_modules")); err != nil {
		t.Fatal(err)
	}
	// No .gitignore — node_modules is untracked and not ignored.
	if err := fixtures.GitInit(root, "package.json"); err != nil {
		t.Fatal(err)
	}

	p := buildPlan(t, root)
	d := mustDecision(t, p, "node_modules")
	if d.Verdict != plan.VerdictSkipped {
		t.Fatalf("want skipped, got %v", d.Verdict)
	}
	if d.Reason != rules.ReasonNotIgnored {
		t.Fatalf("want %q, got %q", rules.ReasonNotIgnored, d.Reason)
	}

	var buf bytes.Buffer
	if err := ui.Render(&buf, p, ui.RenderOptions{NoSize: true, NoColor: true}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Skipped (") {
		t.Fatalf("missing Skipped section:\n%s", out)
	}
	if !strings.Contains(out, rules.ReasonNotIgnored) {
		t.Fatalf("missing reason:\n%s", out)
	}
	if !strings.Contains(out, "→ add `node_modules` to .gitignore") {
		t.Fatalf("missing hint:\n%s", out)
	}
}

func TestGit_Tracked_Skipped(t *testing.T) {
	root := t.TempDir()
	if err := fixtures.WriteFile(filepath.Join(root, "package.json"), `{"name":"app"}`); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.WriteFile(filepath.Join(root, "vite.config.ts"), `export default {}`); err != nil {
		t.Fatal(err)
	}
	distFile := filepath.Join(root, "dist", "index.js")
	if err := fixtures.WriteFile(distFile, "console.log(1)\n"); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.Mkdir(filepath.Join(root, "node_modules", ".vite")); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.WriteFile(filepath.Join(root, ".gitignore"), "node_modules/\n"); err != nil {
		t.Fatal(err)
	}
	// Commit dist — tracked by git.
	if err := fixtures.GitInit(root, "package.json", "vite.config.ts", ".gitignore", "dist"); err != nil {
		t.Fatal(err)
	}

	p := buildPlan(t, root)
	d := mustDecision(t, p, "dist")
	if d.Verdict != plan.VerdictSkipped {
		t.Fatalf("want skipped, got %v (%s)", d.Verdict, d.Reason)
	}
	if d.Reason != rules.ReasonTracked {
		t.Fatalf("want %q, got %q", rules.ReasonTracked, d.Reason)
	}
}

func TestGit_UncommittedChanges_Skipped(t *testing.T) {
	root := t.TempDir()
	if err := fixtures.WriteFile(filepath.Join(root, "package.json"), `{"name":"app"}`); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.WriteFile(filepath.Join(root, "vite.config.ts"), `export default {}`); err != nil {
		t.Fatal(err)
	}
	distFile := filepath.Join(root, "dist", "index.js")
	if err := fixtures.WriteFile(distFile, "v1\n"); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.Mkdir(filepath.Join(root, "node_modules", ".vite")); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.WriteFile(filepath.Join(root, ".gitignore"), "node_modules/\n"); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.GitInit(root, "package.json", "vite.config.ts", ".gitignore", "dist"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(distFile, []byte("v2-dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := buildPlan(t, root)
	d := mustDecision(t, p, "dist")
	if d.Verdict != plan.VerdictSkipped {
		t.Fatalf("want skipped, got %v (%s)", d.Verdict, d.Reason)
	}
	if d.Reason != rules.ReasonUncommitted {
		t.Fatalf("want %q, got %q", rules.ReasonUncommitted, d.Reason)
	}
}

func TestHintForReason(t *testing.T) {
	if g := rules.HintForReason(rules.ReasonTracked, "dist"); g != "intentional? use `keep:` to silence" {
		t.Fatalf("tracked hint: %q", g)
	}
	if g := rules.HintForReason(rules.ReasonUncommitted, "target"); g != "commit or stash first" {
		t.Fatalf("dirty hint: %q", g)
	}
	if g := rules.HintForReason(rules.ReasonNotIgnored, "node_modules"); g != "add `node_modules` to .gitignore" {
		t.Fatalf("not-ignored hint: %q", g)
	}
	if g := rules.HintForReason("orphaned: no validated project", "node_modules"); g != "" {
		t.Fatalf("non-git reason should have empty hint, got %q", g)
	}
}

func buildPlan(t *testing.T, root string) *plan.Plan {
	t.Helper()
	res, err := scan.Walk(scan.Options{Root: root, MaxDepth: 8})
	if err != nil {
		t.Fatal(err)
	}
	return plan.Build(res)
}

func mustDecision(t *testing.T, p *plan.Plan, relBase string) plan.Decision {
	t.Helper()
	for _, d := range p.Decisions {
		if d.Target.RelPath == relBase || filepath.Base(d.Target.Path) == relBase {
			return d
		}
	}
	t.Fatalf("no decision for %q among %+v", relBase, p.Decisions)
	return plan.Decision{}
}
