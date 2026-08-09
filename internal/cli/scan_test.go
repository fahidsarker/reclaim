package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fahid/reclaim/internal/detect"
	"github.com/fahid/reclaim/testdata/fixtures"
)

func TestMain(m *testing.M) {
	detect.MustLoadEmbedded()
	os.Exit(m.Run())
}

func withStateHome(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
}

func nodeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := fixtures.WriteFile(filepath.Join(root, "package.json"), `{"name":"app"}`); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.Mkdir(filepath.Join(root, "node_modules", "x")); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.WriteFile(filepath.Join(root, ".gitignore"), "node_modules/\n"); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRunScan_DeclineExit3(t *testing.T) {
	withStateHome(t)
	root := nodeFixture(t)

	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("n\n"))
	cmd.SetArgs([]string{"scan", "--no-color", root})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error on decline")
	}
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 3 {
		t.Fatalf("want ExitError 3, got %v", err)
	}
	if !strings.Contains(out.String(), "Proceed?") {
		t.Fatalf("expected prompt in output:\n%s", out.String())
	}
	if _, err := os.Lstat(filepath.Join(root, "node_modules")); err != nil {
		t.Fatal("decline must not delete")
	}
}

func TestRunScan_YesDeletes(t *testing.T) {
	withStateHome(t)
	root := nodeFixture(t)

	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs([]string{"scan", "-y", "--no-color", "--no-size", root})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out.String(), "Proceed?") {
		t.Fatalf("-y should skip prompt:\n%s", out.String())
	}
	if _, err := os.Lstat(filepath.Join(root, "node_modules")); !os.IsNotExist(err) {
		t.Fatalf("node_modules should be deleted: %v", err)
	}
	if !strings.Contains(out.String(), "removed") {
		t.Fatalf("expected summary in output:\n%s", out.String())
	}
}

func TestRunScan_DryRunDoesNotDelete(t *testing.T) {
	withStateHome(t)
	root := nodeFixture(t)

	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"scan", "--dry-run", "--no-color", "--no-size", root})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, "node_modules")); err != nil {
		t.Fatal("dry-run must not delete")
	}
}

func TestRunPlan_NoPrompt(t *testing.T) {
	withStateHome(t)
	root := nodeFixture(t)

	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("y\n"))
	cmd.SetArgs([]string{"plan", "--no-color", "--no-size", root})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "Proceed?") {
		t.Fatalf("plan must not prompt:\n%s", out.String())
	}
	if _, err := os.Lstat(filepath.Join(root, "node_modules")); err != nil {
		t.Fatal("plan must not delete")
	}
}
