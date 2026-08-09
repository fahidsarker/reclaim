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

func TestRunScan_DeclineExit3(t *testing.T) {
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
}

func TestRunScan_YesNotImplemented(t *testing.T) {
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

	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("")) // must not be read
	cmd.SetArgs([]string{"scan", "-y", "--no-color", "--no-size", root})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected not-implemented error")
	}
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 1 {
		t.Fatalf("want ExitError 1, got %v", err)
	}
	if !strings.Contains(ee.Message, "deletion is not implemented") {
		t.Fatalf("unexpected message: %v", err)
	}
	if strings.Contains(out.String(), "Proceed?") {
		t.Fatalf("-y should skip prompt:\n%s", out.String())
	}
}

func TestRunPlan_NoPrompt(t *testing.T) {
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
}
