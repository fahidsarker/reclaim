package scan_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fahid/reclaim/internal/scan"
)

func TestWalk_FollowSymlinksCycle(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	if err := os.MkdirAll(a, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(b, 0o755); err != nil {
		t.Fatal(err)
	}
	// a/link -> b, b/link -> a
	if err := os.Symlink(b, filepath.Join(a, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(a, filepath.Join(b, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a, "package.json"), []byte(`{"name":"a"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	var walkErr error
	go func() {
		defer close(done)
		_, walkErr = scan.Walk(scan.Options{
			Root:           root,
			MaxDepth:       20,
			FollowSymlinks: true,
		})
	}()

	select {
	case <-done:
		if walkErr != nil {
			t.Fatal(walkErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("follow-symlinks cycle hung")
	}
}

func TestWalk_Exclude(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	skip := filepath.Join(root, "skipme")
	for _, dir := range []string{app, skip} {
		if err := os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"x"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	res, err := scan.Walk(scan.Options{
		Root:     root,
		MaxDepth: 8,
		Exclude:  []string{"skipme", "skipme/**"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Candidates {
		if c.Project != nil && c.Project.Root == skip {
			t.Fatalf("excluded project still present: %+v", c)
		}
		if c.Target.Path == filepath.Join(skip, "node_modules") {
			t.Fatalf("excluded target present: %s", c.Target.Path)
		}
	}
	if len(res.Projects) == 0 {
		t.Fatal("expected app project")
	}
}

func TestWalk_FrameworkFilter(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte("[package]\nname=\"r\"\nversion=\"0.1.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "target"), 0o755); err != nil {
		t.Fatal(err)
	}

	res, err := scan.Walk(scan.Options{
		Root:       root,
		MaxDepth:   2,
		Frameworks: []string{"rust"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range res.Projects {
		if p.Framework == "nodejs" {
			t.Fatal("nodejs should be filtered out by --framework rust")
		}
	}
}
