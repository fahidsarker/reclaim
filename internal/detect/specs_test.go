package detect_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/fahid/reclaim/internal/detect"
	"github.com/fahid/reclaim/internal/plan"
	"github.com/fahid/reclaim/internal/scan"
	"github.com/fahid/reclaim/testdata/fixtures"
)

func relPaths(targets []detect.Target) []string {
	out := make([]string, len(targets))
	for i, t := range targets {
		out[i] = filepath.ToSlash(t.RelPath)
	}
	sort.Strings(out)
	return out
}

func detectDir(t *testing.T, dir string) *detect.Match {
	t.Helper()
	cache := scan.NewDirCache()
	ctx := &detect.Context{Cache: cache}
	m, err := detect.DetectBest(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSpecs_Table(t *testing.T) {
	cases := []struct {
		name           string
		setup          func(dir string) error
		decoyArtifacts []string
		wantFramework  string
		wantRels       []string // strong match target RelPaths (pre-existence filter)
	}{
		{
			name:          "nodejs",
			setup:         fixtures.NodeJS,
			decoyArtifacts: []string{"node_modules"},
			wantFramework: "nodejs",
			wantRels:      []string{"node_modules"},
		},
		{
			name:          "nextjs",
			setup:         fixtures.NextJS,
			decoyArtifacts: []string{".next", "node_modules"},
			wantFramework: "nextjs",
			wantRels:      []string{".next", "node_modules"},
		},
		{
			name:          "vite",
			setup:         fixtures.Vite,
			decoyArtifacts: []string{"dist", "node_modules"},
			wantFramework: "vite", // DetectBest label; nodejs may also match and union
			wantRels:      []string{"dist", "node_modules", "node_modules/.vite"},
		},
		{
			name:          "turborepo",
			setup:         fixtures.Turborepo,
			decoyArtifacts: []string{".turbo"},
			wantFramework: "turborepo",
			wantRels:      []string{".turbo", "node_modules"},
		},
		{
			name:          "python",
			setup:         fixtures.Python,
			decoyArtifacts: []string{"build", "dist", ".pytest_cache"},
			wantFramework: "python",
			wantRels: []string{
				"*.egg-info",
				"**/__pycache__",
				".ipynb_checkpoints",
				".mypy_cache",
				".pytest_cache",
				".ruff_cache",
				"build",
				"dist",
			},
		},
		{
			name:          "rust",
			setup:         fixtures.Rust,
			decoyArtifacts: []string{"target"},
			wantFramework: "rust",
			wantRels:      []string{"target"},
		},
		{
			name:          "go",
			setup:         fixtures.Go,
			decoyArtifacts: []string{"vendor", "bin", "dist"},
			wantFramework: "go",
			wantRels:      []string{"bin", "dist", "vendor"},
		},
		{
			name:          "maven",
			setup:         fixtures.Maven,
			decoyArtifacts: []string{"target"},
			wantFramework: "maven",
			wantRels:      []string{"target"},
		},
		{
			name:          "gradle",
			setup:         fixtures.Gradle,
			decoyArtifacts: []string{"build", ".gradle"},
			wantFramework: "gradle",
			wantRels:      []string{".gradle", "build"},
		},
		{
			name:          "flutter",
			setup:         fixtures.Flutter,
			decoyArtifacts: []string{"build", ".dart_tool"},
			wantFramework: "flutter",
			wantRels: []string{
				".dart_tool",
				".flutter-plugins",
				".flutter-plugins-dependencies",
				"android/.gradle",
				"build",
				"ios/.symlinks",
				"ios/Pods",
				"macos/Pods",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/positive", func(t *testing.T) {
			dir := t.TempDir()
			if err := tc.setup(dir); err != nil {
				t.Fatal(err)
			}
			m := detectDir(t, dir)
			if m == nil || m.Confidence != detect.ConfidenceStrong {
				t.Fatalf("want strong match, got %+v", m)
			}
			if m.Framework != tc.wantFramework {
				t.Fatalf("framework: got %q want %q", m.Framework, tc.wantFramework)
			}
			got := relPaths(m.Targets)
			want := append([]string(nil), tc.wantRels...)
			sort.Strings(want)
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("targets:\n got %v\nwant %v", got, want)
			}
		})

		t.Run(tc.name+"/decoy", func(t *testing.T) {
			dir := t.TempDir()
			if err := fixtures.Decoy(dir, tc.decoyArtifacts...); err != nil {
				t.Fatal(err)
			}
			m := detectDir(t, dir)
			if m != nil && m.Confidence == detect.ConfidenceStrong {
				t.Fatalf("decoy must not be strong project, got %+v", m)
			}
		})
	}
}

func TestNextJS_ExtendsUnion(t *testing.T) {
	dir := t.TempDir()
	if err := fixtures.NextJS(dir); err != nil {
		t.Fatal(err)
	}
	m := detectDir(t, dir)
	if m == nil || m.Framework != "nextjs" {
		t.Fatalf("want nextjs, got %+v", m)
	}
	got := map[string]bool{}
	for _, tgt := range m.Targets {
		got[filepath.ToSlash(tgt.RelPath)] = true
	}
	for _, want := range []string{"node_modules", ".next"} {
		if !got[want] {
			t.Fatalf("missing %s in %+v", want, relPaths(m.Targets))
		}
	}
}

func TestLoadSpecs_Malformed(t *testing.T) {
	dir := t.TempDir()
	bad := []byte("name: broken\ndetect:\n  unknown_pred: true\n")
	if err := os.WriteFile(filepath.Join(dir, "broken.yaml"), bad, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := detect.LoadSpecsFromFS(os.DirFS(dir))
	if err == nil {
		t.Fatal("expected malformed spec to fail")
	}
}

func TestLoadEmbeddedSpecs(t *testing.T) {
	specs := detect.EmbeddedSpecs()
	if len(specs) != 10 {
		t.Fatalf("want 10 embedded specs, got %d", len(specs))
	}
}

func TestMixedFixtures_Plan(t *testing.T) {
	root := t.TempDir()
	projects := map[string]func(string) error{
		"node-app":   fixtures.NodeJS,
		"next-app":   fixtures.NextJS,
		"vite-app":   fixtures.Vite,
		"turbo-app":  fixtures.Turborepo,
		"py-app":     fixtures.Python,
		"rust-app":   fixtures.Rust,
		"go-app":     fixtures.Go,
		"maven-app":  fixtures.Maven,
		"gradle-app": fixtures.Gradle,
		"flutter-app": fixtures.Flutter,
	}
	for name, setup := range projects {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := setup(dir); err != nil {
			t.Fatal(err)
		}
	}

	res, err := scan.Walk(scan.Options{Root: root, MaxDepth: 8})
	if err != nil {
		t.Fatal(err)
	}
	p := plan.Build(res)

	frameworks := map[string]bool{}
	var deletes, skips int
	aggressiveSkips := 0
	for _, d := range p.Decisions {
		if d.Project != nil {
			frameworks[d.Project.Framework] = true
		}
		switch d.Verdict {
		case plan.VerdictDelete:
			deletes++
		case plan.VerdictSkipped:
			skips++
			if d.Reason == "requires --aggressive" {
				aggressiveSkips++
			}
		}
	}

	for _, want := range []string{"nodejs", "nextjs", "vite", "turborepo", "python", "rust", "go", "maven", "gradle", "flutter"} {
		if !frameworks[want] {
			t.Errorf("missing framework %s in plan (seen %v)", want, frameworks)
		}
	}
	if deletes == 0 {
		t.Fatal("expected some delete candidates")
	}
	if aggressiveSkips == 0 {
		t.Fatal("expected SafetyRequiresFlag targets skipped with requires --aggressive")
	}

	// next-app should contribute node_modules + .next as deletes
	var nextDeletes []string
	for _, d := range p.Decisions {
		if d.Verdict != plan.VerdictDelete || d.Project == nil {
			continue
		}
		if filepath.Base(d.Project.Root) != "next-app" {
			continue
		}
		nextDeletes = append(nextDeletes, filepath.ToSlash(d.Target.RelPath))
	}
	sort.Strings(nextDeletes)
	if !containsAll(nextDeletes, "node_modules", ".next") {
		t.Fatalf("next-app deletes = %v, want node_modules and .next", nextDeletes)
	}
}

func containsAll(have []string, want ...string) bool {
	set := map[string]bool{}
	for _, h := range have {
		set[h] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}
