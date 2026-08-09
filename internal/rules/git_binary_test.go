package rules_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fahid/reclaim/internal/rules"
	"github.com/fahid/reclaim/testdata/fixtures"
)

func TestInspectGit_NotIgnored(t *testing.T) {
	root := t.TempDir()
	if err := fixtures.WriteFile(filepath.Join(root, "package.json"), `{"name":"x"}`); err != nil {
		t.Fatal(err)
	}
	nm := filepath.Join(root, "node_modules")
	if err := fixtures.Mkdir(nm); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.GitInit(root, "package.json"); err != nil {
		t.Fatal(err)
	}

	repo, err := rules.FindGitRepo(root)
	if err != nil || repo == nil {
		t.Fatalf("repo: %v", err)
	}
	ins := rules.InspectGit(repo, nm, rules.GitOptions{})
	if ins.Ignored {
		t.Fatal("expected not ignored")
	}
	if ins.Tracked {
		t.Fatal("expected not tracked")
	}
	if reason := rules.CheckGit(repo, nm); reason != rules.ReasonNotIgnored {
		t.Fatalf("CheckGit=%q", reason)
	}
}

func TestCheckGit_UseBinary(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	root := t.TempDir()
	if err := fixtures.WriteFile(filepath.Join(root, "package.json"), `{"name":"x"}`); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.WriteFile(filepath.Join(root, ".gitignore"), "node_modules/\n"); err != nil {
		t.Fatal(err)
	}
	nm := filepath.Join(root, "node_modules")
	if err := fixtures.Mkdir(nm); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.GitInit(root, "package.json", ".gitignore"); err != nil {
		t.Fatal(err)
	}

	repo, err := rules.FindGitRepo(root)
	if err != nil || repo == nil {
		t.Fatalf("repo: %v", err)
	}
	if reason := rules.CheckGitOpts(repo, nm, rules.GitOptions{UseBinary: true}); reason != "" {
		t.Fatalf("ignored path should allow delete, got %q", reason)
	}
	ins := rules.InspectGit(repo, nm, rules.GitOptions{UseBinary: true})
	if !ins.Ignored {
		t.Fatal("expected ignored via git binary")
	}
	_ = os.Stdout
}
