package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fahid/reclaim/internal/rules"
)

func TestGuardRoot_Home(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	err = rules.GuardRoot(home, false)
	if err == nil {
		t.Fatal("expected error scanning $HOME without acknowledgement")
	}
	if err := rules.GuardRoot(home, true); err != nil {
		t.Fatalf("expected allow with acknowledgement: %v", err)
	}
}

func TestGuardRoot_FilesystemRoot(t *testing.T) {
	err := rules.GuardRoot("/", false)
	if err == nil {
		t.Fatal("expected error scanning / without acknowledgement")
	}
}

func TestGuardRoot_NormalDir(t *testing.T) {
	dir := t.TempDir()
	if err := rules.GuardRoot(dir, false); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCheckTarget_VCS(t *testing.T) {
	root := t.TempDir()
	gitPath := filepath.Join(root, "proj", ".git")
	if err := os.MkdirAll(gitPath, 0o755); err != nil {
		t.Fatal(err)
	}
	reason := rules.CheckTarget(root, gitPath)
	if reason == "" {
		t.Fatal("expected .git target to be rejected")
	}
}

func TestCheckTarget_SymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	outsideNM := filepath.Join(outside, "secret")
	if err := os.MkdirAll(outsideNM, 0o755); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(root, "node_modules")
	if err := os.Symlink(outsideNM, link); err != nil {
		t.Fatal(err)
	}

	reason := rules.CheckTarget(root, link)
	if reason == "" {
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestCheckTarget_SafeUnderRoot(t *testing.T) {
	root := t.TempDir()
	nm := filepath.Join(root, "app", "node_modules")
	if err := os.MkdirAll(nm, 0o755); err != nil {
		t.Fatal(err)
	}
	if reason := rules.CheckTarget(root, nm); reason != "" {
		t.Fatalf("unexpected reject: %s", reason)
	}
}

func TestCheckTarget_StateDenylist(t *testing.T) {
	root := t.TempDir()
	envFile := filepath.Join(root, ".env.local")
	if err := os.WriteFile(envFile, []byte("X=1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if reason := rules.CheckTarget(root, envFile); reason == "" {
		t.Fatal("expected .env* to be rejected")
	}
}

func TestIsVCSDir(t *testing.T) {
	for _, name := range []string{".git", ".hg", ".svn", ".jj"} {
		if !rules.IsVCSDir(name) {
			t.Fatalf("expected %q to be VCS", name)
		}
	}
	if rules.IsVCSDir("node_modules") {
		t.Fatal("node_modules should not be VCS")
	}
}
