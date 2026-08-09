package plan_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fahid/reclaim/internal/detect"
	"github.com/fahid/reclaim/internal/plan"
	"github.com/fahid/reclaim/internal/rules"
	"github.com/fahid/reclaim/internal/scan"
	"github.com/fahid/reclaim/testdata/fixtures"
)

func TestMain(m *testing.M) {
	detect.MustLoadEmbedded()
	os.Exit(m.Run())
}

func TestControl_KeepBeatsDetector(t *testing.T) {
	root := t.TempDir()
	if err := fixtures.NodeJS(root); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.WriteFile(filepath.Join(root, ".reclaim.yaml"), `
version: 1
keep:
  - node_modules
`); err != nil {
		t.Fatal(err)
	}

	p := buildPlan(t, root, false)
	d := mustDecision(t, p, "node_modules")
	if d.Verdict != plan.VerdictKept {
		t.Fatalf("want kept, got %v (%s)", d.Verdict, d.Reason)
	}
}

func TestControl_NoConfigIgnoresKeep(t *testing.T) {
	root := t.TempDir()
	if err := fixtures.NodeJS(root); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.WriteFile(filepath.Join(root, ".reclaim.yaml"), `
keep:
  - node_modules
`); err != nil {
		t.Fatal(err)
	}

	p := buildPlan(t, root, true)
	d := mustDecision(t, p, "node_modules")
	if d.Verdict != plan.VerdictDelete {
		t.Fatalf("want delete with --no-config, got %v (%s)", d.Verdict, d.Reason)
	}
}

func TestControl_InheritAppliesToChildren(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "packages", "api")
	if err := fixtures.NodeJS(child); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.WriteFile(filepath.Join(root, ".reclaim.yaml"), `
version: 1
inherit: true
keep:
  - node_modules
`); err != nil {
		t.Fatal(err)
	}

	p := buildPlan(t, root, false)
	d := mustDecision(t, p, "node_modules")
	if d.Verdict != plan.VerdictKept {
		t.Fatalf("inherited keep want kept, got %v (%s)", d.Verdict, d.Reason)
	}
}

func TestControl_StrictIgnoresDetectors(t *testing.T) {
	root := t.TempDir()
	if err := fixtures.NodeJS(root); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.Mkdir(filepath.Join(root, "custom_cache")); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.WriteFile(filepath.Join(root, ".reclaim.yaml"), `
mode: strict
delete:
  - custom_cache
`); err != nil {
		t.Fatal(err)
	}

	p := buildPlan(t, root, false)

	var sawNM, sawCustom bool
	for _, d := range p.Decisions {
		base := filepath.Base(d.Target.Path)
		switch base {
		case "node_modules":
			sawNM = true
		case "custom_cache":
			sawCustom = true
			if d.Verdict != plan.VerdictDelete {
				t.Fatalf("custom_cache want delete, got %v (%s)", d.Verdict, d.Reason)
			}
		}
	}
	if sawNM {
		t.Fatal("strict mode must ignore detector node_modules")
	}
	if !sawCustom {
		t.Fatal("strict mode must include explicit delete: custom_cache")
	}
}

func TestControl_RequireGitIgnoredFalse(t *testing.T) {
	root := t.TempDir()
	if err := fixtures.WriteFile(filepath.Join(root, "package.json"), `{"name":"app"}`); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.Mkdir(filepath.Join(root, "node_modules")); err != nil {
		t.Fatal(err)
	}
	// No .gitignore — would normally skip as not ignored.
	if err := fixtures.GitInit(root, "package.json"); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.WriteFile(filepath.Join(root, ".reclaim.yaml"), `
require_git_ignored: false
`); err != nil {
		t.Fatal(err)
	}

	p := buildPlan(t, root, false)
	d := mustDecision(t, p, "node_modules")
	if d.Verdict != plan.VerdictDelete {
		t.Fatalf("want delete with require_git_ignored:false, got %v (%s)", d.Verdict, d.Reason)
	}
}

func TestControl_ExplicitDeleteBypassesGit(t *testing.T) {
	root := t.TempDir()
	if err := fixtures.WriteFile(filepath.Join(root, "package.json"), `{"name":"app"}`); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.Mkdir(filepath.Join(root, "node_modules")); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.GitInit(root, "package.json"); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.WriteFile(filepath.Join(root, ".reclaim.yaml"), `
delete:
  - node_modules
`); err != nil {
		t.Fatal(err)
	}

	p := buildPlan(t, root, false)
	d := mustDecision(t, p, "node_modules")
	if d.Verdict != plan.VerdictDelete {
		t.Fatalf("want delete via explicit delete:, got %v (%s)", d.Verdict, d.Reason)
	}
	if d.Reason == rules.ReasonNotIgnored {
		t.Fatal("explicit delete should bypass git not-ignored veto")
	}
}

func TestControl_TraversalPatternRejected(t *testing.T) {
	root := t.TempDir()
	if err := fixtures.NodeJS(root); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.WriteFile(filepath.Join(root, ".reclaim.yaml"), `
delete:
  - ../outside
`); err != nil {
		t.Fatal(err)
	}

	_, err := scan.Walk(scan.Options{Root: root, MaxDepth: 8})
	if err == nil {
		t.Fatal("expected error for .. in control pattern")
	}
}

func TestControl_IgnorePrunesSubtree(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "skipme")
	if err := fixtures.NodeJS(child); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.WriteFile(filepath.Join(child, ".reclaim.yaml"), `
ignore: true
`); err != nil {
		t.Fatal(err)
	}

	p := buildPlan(t, root, false)
	for _, d := range p.Decisions {
		if filepath.Base(d.Target.Path) == "node_modules" {
			t.Fatalf("ignored subtree should not produce candidates, got %v", d)
		}
	}
}

func buildPlan(t *testing.T, root string, noConfig bool) *plan.Plan {
	t.Helper()
	res, err := scan.Walk(scan.Options{Root: root, MaxDepth: 8, NoConfig: noConfig})
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
	t.Fatalf("no decision for %q among %+v", relBase, decisionsSummary(p))
	return plan.Decision{}
}

func decisionsSummary(p *plan.Plan) []string {
	var out []string
	for _, d := range p.Decisions {
		out = append(out, d.Target.Path+":"+d.Verdict.String()+":"+d.Reason)
	}
	return out
}
