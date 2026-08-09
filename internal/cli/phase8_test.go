package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fahid/reclaim/internal/detect"
	_ "github.com/fahid/reclaim/internal/detect/builtin"
	"github.com/fahid/reclaim/internal/plan"
	"github.com/fahid/reclaim/internal/rules"
	"github.com/fahid/reclaim/testdata/fixtures"
)

func TestExplain_SkippedGit(t *testing.T) {
	detect.MustLoadEmbedded()
	root := t.TempDir()
	if err := fixtures.WriteFile(filepath.Join(root, "package.json"), `{"name":"legacy"}`); err != nil {
		t.Fatal(err)
	}
	nm := filepath.Join(root, "node_modules")
	if err := fixtures.Mkdir(nm); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.GitInit(root, "package.json"); err != nil {
		t.Fatal(err)
	}

	ex, err := plan.ExplainPath(nm, plan.ExplainOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if ex.Verdict != plan.VerdictSkipped {
		t.Fatalf("verdict=%s want skipped", ex.Verdict)
	}
	if ex.Reason != rules.ReasonNotIgnored {
		t.Fatalf("reason=%q want %q", ex.Reason, rules.ReasonNotIgnored)
	}
	out := plan.FormatExplanation(ex)
	if !strings.Contains(out, "SKIPPED") {
		t.Fatalf("explain missing SKIPPED:\n%s", out)
	}
	if !strings.Contains(out, "not in .gitignore") {
		t.Fatalf("explain missing git reason:\n%s", out)
	}
	if !strings.Contains(out, "← blocking") {
		t.Fatalf("explain missing blocking marker:\n%s", out)
	}
}

func TestDetectorsTest_PredicateTree(t *testing.T) {
	detect.MustLoadEmbedded()
	root := t.TempDir()
	if err := fixtures.NodeJS(root); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"detectors", "test", root})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "nodejs: MATCH") {
		t.Fatalf("expected nodejs match:\n%s", out)
	}
	if !strings.Contains(out, "[pass] file_exists: package.json") && !strings.Contains(out, "file_exists: package.json") {
		t.Fatalf("expected predicate tree:\n%s", out)
	}
}

func TestDetectorsListAndShow(t *testing.T) {
	detect.MustLoadEmbedded()
	cmd := newRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(os.Stderr)
	cmd.SetArgs([]string{"detectors", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "nodejs") {
		t.Fatalf("list missing nodejs:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "builtin") {
		t.Fatalf("list missing builtin source:\n%s", buf.String())
	}

	buf.Reset()
	cmd = newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"detectors", "show", "nextjs"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	show := buf.String()
	if !strings.Contains(show, "name: nextjs") {
		t.Fatalf("show missing name:\n%s", show)
	}
	if !strings.Contains(show, "node_modules") || !strings.Contains(show, ".next") {
		t.Fatalf("show missing resolved targets:\n%s", show)
	}
}

func TestInit_Scaffold(t *testing.T) {
	detect.MustLoadEmbedded()
	dir := t.TempDir()
	if err := fixtures.NodeJS(dir); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	cmd := newRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"init", "--no-size", "--no-color"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".reclaim.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if !strings.Contains(body, "# keep:") {
		t.Fatalf("scaffold missing commented keep:\n%s", body)
	}
	if !strings.Contains(body, "node_modules") {
		t.Fatalf("scaffold missing node_modules:\n%s", body)
	}

	// Second init should fail.
	cmd = newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"init"})
	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected error when .reclaim.yaml exists")
	}
	if ExitCode(err) != 2 {
		t.Fatalf("exit code=%d want 2", ExitCode(err))
	}
}

func TestPlan_JSON(t *testing.T) {
	detect.MustLoadEmbedded()
	root := t.TempDir()
	if err := fixtures.NodeJS(root); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"plan", "--json", "--no-size", "--no-color", root})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{`"version": 1`, `"root"`, `"projects"`, `"skipped"`, `"totals"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("json missing %s:\n%s", want, out)
		}
	}
}
