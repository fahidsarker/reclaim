package detect_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fahid/reclaim/internal/detect"
	_ "github.com/fahid/reclaim/internal/detect/builtin"
	"github.com/fahid/reclaim/internal/scan"
)

func TestNodeJS_Strong(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"app"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cache := scan.NewDirCache()
	ctx := &detect.Context{Cache: cache}
	m, err := detect.DetectBest(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil || m.Confidence != detect.ConfidenceStrong {
		t.Fatalf("want strong match, got %+v", m)
	}
	if len(m.Targets) != 1 || m.Targets[0].RelPath != "node_modules" {
		t.Fatalf("targets: %+v", m.Targets)
	}
}

func TestNodeJS_WeakCorrupt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`nope`), 0o644); err != nil {
		t.Fatal(err)
	}
	cache := scan.NewDirCache()
	ctx := &detect.Context{Cache: cache}
	m, err := detect.DetectBest(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil || m.Confidence != detect.ConfidenceWeak {
		t.Fatalf("want weak match, got %+v", m)
	}
	if len(m.Targets) != 0 {
		t.Fatalf("weak must have no targets: %+v", m.Targets)
	}
}

func TestNodeJS_NoManifest(t *testing.T) {
	dir := t.TempDir()
	cache := scan.NewDirCache()
	ctx := &detect.Context{Cache: cache}
	m, err := detect.DetectBest(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if m != nil {
		t.Fatalf("want nil match, got %+v", m)
	}
}
