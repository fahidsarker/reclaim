package builtin

import (
	"path/filepath"
	"strings"

	"github.com/fahid/reclaim/internal/detect"
)

// BazelDetector matches Bazel workspaces and emits bazel-* symlink/dir targets.
type BazelDetector struct{}

func (d *BazelDetector) Name() string  { return "bazel" }
func (d *BazelDetector) Priority() int { return 10 }

func (d *BazelDetector) Detect(ctx *detect.Context, dir string) (*detect.Match, error) {
	var manifest string
	for _, name := range []string{"MODULE.bazel", "WORKSPACE", "WORKSPACE.bazel"} {
		p := filepath.Join(dir, name)
		if existsFile(ctx, p) {
			manifest = p
			break
		}
	}
	if manifest == "" {
		return nil, nil
	}

	return &detect.Match{
		Framework:  "bazel",
		Confidence: detect.ConfidenceStrong,
		Manifest:   manifest,
		Targets:    bazelTargets(ctx, dir),
	}, nil
}

func bazelTargets(ctx *detect.Context, dir string) []detect.Target {
	if ctx == nil || ctx.Cache == nil {
		return nil
	}
	entries, err := ctx.Cache.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []detect.Target
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "bazel-") {
			continue
		}
		out = append(out, detect.Target{
			RelPath:    name,
			Kind:       detect.KindDir,
			Reason:     "Bazel output symlink",
			Regenerate: "bazel build",
		})
	}
	return out
}

func existsFile(ctx *detect.Context, path string) bool {
	if ctx == nil || ctx.Cache == nil {
		return false
	}
	info, err := ctx.Cache.Lstat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
