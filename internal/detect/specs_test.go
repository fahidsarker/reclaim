package detect_test

import (
	"bytes"
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
	pythonRels := []string{
		"*.egg-info",
		"**/__pycache__",
		".ipynb_checkpoints",
		".mypy_cache",
		".pytest_cache",
		".ruff_cache",
		"build",
		"dist",
	}
	cases := []struct {
		name           string
		setup          func(dir string) error
		decoyArtifacts []string
		wantFramework  string
		wantRels       []string
	}{
		{"nodejs", fixtures.NodeJS, []string{"node_modules"}, "nodejs", []string{"node_modules"}},
		{"nextjs", fixtures.NextJS, []string{".next", "node_modules"}, "nextjs", []string{".next", "node_modules"}},
		{"vite", fixtures.Vite, []string{"dist", "node_modules"}, "vite", []string{"dist", "node_modules", "node_modules/.vite"}},
		{"turborepo", fixtures.Turborepo, []string{".turbo"}, "turborepo", []string{".turbo", "node_modules"}},
		{"python", fixtures.Python, []string{"build", "dist", ".pytest_cache"}, "python", pythonRels},
		{"rust", fixtures.Rust, []string{"target"}, "rust", []string{"target"}},
		{"go", fixtures.Go, []string{"vendor", "bin", "dist"}, "go", []string{"bin", "dist", "vendor"}},
		{"maven", fixtures.Maven, []string{"target"}, "maven", []string{"target"}},
		{"gradle", fixtures.Gradle, []string{"build", ".gradle"}, "gradle", []string{".gradle", "build"}},
		{"flutter", fixtures.Flutter, []string{"build", ".dart_tool"}, "flutter", []string{
			".dart_tool", ".flutter-plugins", ".flutter-plugins-dependencies",
			"android/.gradle", "build", "ios/.symlinks", "ios/Pods", "macos/Pods",
		}},
		{"nuxt", fixtures.Nuxt, []string{".nuxt"}, "nuxt", []string{".nuxt", ".output", "node_modules"}},
		{"sveltekit", fixtures.SvelteKit, []string{".svelte-kit"}, "sveltekit", []string{".svelte-kit", "node_modules"}},
		{"astro", fixtures.Astro, []string{".astro", "dist"}, "astro", []string{".astro", "dist", "node_modules"}},
		{"angular", fixtures.Angular, []string{".angular/cache", "dist"}, "angular", []string{".angular/cache", "dist", "node_modules"}},
		{"gatsby", fixtures.Gatsby, []string{".cache", "public"}, "gatsby", []string{".cache", "node_modules", "public"}},
		{"remix", fixtures.Remix, []string{"build", ".cache"}, "remix", []string{".cache", "build", "node_modules"}},
		{"nx", fixtures.Nx, []string{".nx/cache", "dist"}, "nx", []string{".nx/cache", "dist", "node_modules"}},
		{"parcel", fixtures.Parcel, []string{".parcel-cache", "dist"}, "parcel", []string{".parcel-cache", "dist", "node_modules"}},
		{"electron", fixtures.Electron, []string{"dist", "out"}, "electron", []string{"dist", "node_modules", "out", "release"}},
		{"expo", fixtures.Expo, []string{".expo"}, "expo", []string{".expo", ".expo-shared", "node_modules"}},
		{"react-native", fixtures.ReactNative, []string{"ios/Pods"}, "react-native", []string{
			"android/.gradle", "android/build", "ios/Pods", "ios/build", "node_modules",
		}},
		{"storybook", fixtures.Storybook, []string{"storybook-static"}, "storybook", []string{"node_modules", "storybook-static"}},
		{"jest", fixtures.Jest, []string{"coverage"}, "jest", []string{"coverage", "node_modules"}},
		{"python-venv", fixtures.PythonVenv, []string{".venv"}, "python-venv", []string{".venv", "env", "venv"}},
		{"tox", fixtures.Tox, []string{".tox"}, "tox", []string{".tox"}},
		{"poetry", fixtures.Poetry, []string{"dist"}, "poetry", pythonRels},
		{"uv", fixtures.UV, []string{"dist"}, "uv", nil},
		{"android", fixtures.Android, []string{"app/build"}, "android", []string{".cxx", ".gradle", "app/build", "build", "captures"}},
		{"sbt", fixtures.SBT, []string{"target"}, "sbt", []string{"project/target", "target"}},
		{"dart", fixtures.Dart, []string{"build", ".dart_tool"}, "dart", []string{".dart_tool", "build"}},
		{"swiftpm", fixtures.SwiftPM, []string{".build"}, "swiftpm", []string{".build", ".swiftpm"}},
		{"cocoapods", fixtures.CocoaPods, []string{"Pods"}, "cocoapods", []string{"Pods"}},
		{"carthage", fixtures.Carthage, []string{"Carthage/Build"}, "carthage", []string{"Carthage/Build"}},
		{"xcode", fixtures.Xcode, []string{"build", "DerivedData"}, "xcode", []string{"DerivedData", "build"}},
		{"dotnet", fixtures.Dotnet, []string{"bin", "obj"}, "dotnet", []string{"bin", "obj", "packages"}},
		{"bundler", fixtures.Bundler, []string{"vendor/bundle"}, "bundler", []string{".bundle", "tmp/cache", "vendor/bundle"}},
		{"rails", fixtures.Rails, []string{"tmp/cache", "log"}, "rails", []string{
			".bundle", "log", "public/assets", "public/packs", "tmp/cache", "vendor/bundle",
		}},
		{"composer", fixtures.Composer, []string{"vendor"}, "composer", []string{"vendor"}},
		{"laravel", fixtures.Laravel, []string{"bootstrap/cache"}, "laravel", []string{
			"bootstrap/cache", "storage/framework/cache", "storage/framework/sessions", "storage/framework/views",
		}},
		{"symfony", fixtures.Symfony, []string{"var/cache"}, "symfony", []string{"var/cache", "var/log"}},
		{"cmake", fixtures.CMake, []string{"build"}, "cmake", []string{"CMakeFiles", "_build", "build", "cmake-build-*"}},
		{"meson", fixtures.Meson, []string{"build", "builddir"}, "meson", []string{"build", "builddir"}},
		{"bazel", fixtures.Bazel, []string{"bazel-bin"}, "bazel", []string{"bazel-bin", "bazel-out"}},
		{"zig", fixtures.Zig, []string{"zig-out"}, "zig", []string{".zig-cache", "zig-cache", "zig-out"}},
		{"elixir", fixtures.Elixir, []string{"_build", "deps"}, "elixir", []string{"_build", "deps"}},
		{"haskell", fixtures.Haskell, []string{"dist-newstyle"}, "haskell", []string{".stack-work", "dist-newstyle"}},
		{"nim", fixtures.Nim, []string{"nimcache"}, "nim", []string{"nimcache"}},
		{"hugo", fixtures.Hugo, []string{"public"}, "hugo", []string{".hugo_build.lock", "public", "resources/_gen"}},
		{"jekyll", fixtures.Jekyll, []string{"_site"}, "jekyll", []string{
			".bundle", ".jekyll-cache", ".sass-cache", "_site", "tmp/cache", "vendor/bundle",
		}},
		{"zola", fixtures.Zola, []string{"public"}, "zola", []string{"public"}},
		{"eleventy", fixtures.Eleventy, []string{"_site"}, "eleventy", []string{"_site", "node_modules"}},
		{"mkdocs", fixtures.MkDocs, []string{"site"}, "mkdocs", []string{"site"}},
		{"unity", fixtures.Unity, []string{"Library"}, "unity", []string{"Build", "Library", "Logs", "Obj", "Temp", "UserSettings"}},
		{"godot", fixtures.Godot, []string{".godot"}, "godot", []string{".godot", ".import"}},
		{"unreal", fixtures.Unreal, []string{"Binaries", "Saved"}, "unreal", []string{"Binaries", "DerivedDataCache", "Intermediate", "Saved"}},
		{"terraform", fixtures.Terraform, []string{".terraform"}, "terraform", []string{".terraform"}},
		{"pulumi", fixtures.Pulumi, []string{".pulumi"}, "pulumi", []string{".pulumi"}},
		{"latex", fixtures.Latex, []string{"main.aux"}, "latex", []string{"*.aux", "*.log", "*.out", "*.toc", "_minted-*"}},
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
	if len(specs) != 56 {
		t.Fatalf("want 56 embedded specs, got %d", len(specs))
	}
}

func TestMixedFixtures_Plan(t *testing.T) {
	root := t.TempDir()
	projects := map[string]func(string) error{
		"node-app":    fixtures.NodeJS,
		"next-app":    fixtures.NextJS,
		"vite-app":    fixtures.Vite,
		"turbo-app":   fixtures.Turborepo,
		"py-app":      fixtures.Python,
		"rust-app":    fixtures.Rust,
		"go-app":      fixtures.Go,
		"maven-app":   fixtures.Maven,
		"gradle-app":  fixtures.Gradle,
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
	p := plan.Build(res, plan.Options{})

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
	_ = skips

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

func TestAggressive_IncludesRequiresFlag(t *testing.T) {
	root := t.TempDir()
	if err := fixtures.Go(root); err != nil {
		t.Fatal(err)
	}
	// Go fixture gitignores bin/dist but not vendor — mark vendor ignored for delete path.
	if err := fixtures.WriteFile(filepath.Join(root, ".gitignore"), "bin/\ndist/\nvendor/\n"); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.GitInit(root, ".gitignore", "go.mod"); err != nil {
		t.Fatal(err)
	}

	res, err := scan.Walk(scan.Options{Root: root, MaxDepth: 8})
	if err != nil {
		t.Fatal(err)
	}

	plain := plan.Build(res, plan.Options{})
	var vendorSkipped bool
	for _, d := range plain.Decisions {
		if filepath.Base(d.Target.Path) == "vendor" && d.Reason == "requires --aggressive" {
			vendorSkipped = true
		}
	}
	if !vendorSkipped {
		t.Fatal("expected vendor skipped without --aggressive")
	}

	agg := plan.Build(res, plan.Options{Aggressive: true})
	var vendorDelete bool
	for _, d := range agg.Decisions {
		if filepath.Base(d.Target.Path) == "vendor" && d.Verdict == plan.VerdictDelete {
			vendorDelete = true
		}
	}
	if !vendorDelete {
		t.Fatal("expected vendor delete with --aggressive")
	}
}

func TestRustWorkspace_MemberTargetSkipped(t *testing.T) {
	root := t.TempDir()
	if err := fixtures.RustWorkspace(root); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.GitInit(root, ".gitignore", "Cargo.toml", "crates/svc/Cargo.toml"); err != nil {
		t.Fatal(err)
	}

	res, err := scan.Walk(scan.Options{Root: root, MaxDepth: 8})
	if err != nil {
		t.Fatal(err)
	}
	p := plan.Build(res, plan.Options{})

	var rootTarget, memberTarget bool
	for _, d := range p.Decisions {
		if d.Verdict != plan.VerdictDelete {
			continue
		}
		if d.Project == nil || filepath.ToSlash(d.Target.RelPath) != "target" {
			continue
		}
		rel, _ := filepath.Rel(root, d.Project.Root)
		switch filepath.ToSlash(rel) {
		case ".":
			rootTarget = true
		case "crates/svc":
			memberTarget = true
		}
	}
	if !rootTarget {
		t.Fatal("expected workspace root target as delete candidate")
	}
	if memberTarget {
		t.Fatal("member crate target should be skipped in favour of workspace root")
	}

	// Member should still be detected as a rust project (strong) without target.
	member := filepath.Join(root, "crates", "svc")
	m := detectDir(t, member)
	if m == nil || m.Framework != "rust" || m.Confidence != detect.ConfidenceStrong {
		t.Fatalf("want strong rust member, got %+v", m)
	}
	if len(m.Targets) != 0 {
		t.Fatalf("member should omit target, got %v", relPaths(m.Targets))
	}
}

func TestLoadUserSpecs_WarnAndSkip(t *testing.T) {
	dir := t.TempDir()
	valid := `
name: custom-tool
description: user spec
priority: 50
detect:
  file_exists: custom.manifest
targets:
  - path: .custom-cache
    reason: custom cache
    regenerate: custom rebuild
`
	if err := os.WriteFile(filepath.Join(dir, "custom.yaml"), []byte(valid), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("name: broken\ndetect:\n  nope: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var warnings bytes.Buffer
	n := detect.LoadUserSpecsFromDir(dir, func(msg string) {
		warnings.WriteString(msg + "\n")
	})
	if n != 1 {
		t.Fatalf("registered %d, want 1", n)
	}
	if !strings.Contains(warnings.String(), "bad.yaml") {
		t.Fatalf("expected warning about bad.yaml, got %q", warnings.String())
	}

	proj := t.TempDir()
	if err := fixtures.WriteFile(filepath.Join(proj, "custom.manifest"), "ok"); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.Mkdir(filepath.Join(proj, ".custom-cache")); err != nil {
		t.Fatal(err)
	}
	m := detectDir(t, proj)
	if m == nil || m.Framework != "custom-tool" {
		t.Fatalf("want custom-tool, got %+v", m)
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
