package builtin

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/fahid/reclaim/internal/detect"
)

// RustDetector matches Cargo projects and attributes target/ to workspace roots.
type RustDetector struct{}

func (d *RustDetector) Name() string  { return "rust" }
func (d *RustDetector) Priority() int { return 10 }

func (d *RustDetector) Detect(ctx *detect.Context, dir string) (*detect.Match, error) {
	manifest := filepath.Join(dir, "Cargo.toml")
	data, err := readFile(ctx, manifest)
	if err != nil {
		return nil, nil
	}
	var cargo cargoFile
	if err := toml.Unmarshal(data, &cargo); err != nil {
		return &detect.Match{
			Framework:  "rust",
			Confidence: detect.ConfidenceWeak,
			Manifest:   manifest,
			Targets:    []detect.Target{rustTarget()},
		}, nil
	}
	if cargo.Package == nil && cargo.Workspace == nil {
		return nil, nil
	}

	m := &detect.Match{
		Framework:  "rust",
		Confidence: detect.ConfidenceStrong,
		Manifest:   manifest,
	}
	if shouldEmitRustTarget(ctx, dir, &cargo) {
		m.Targets = []detect.Target{rustTarget()}
	}
	return m, nil
}

func rustTarget() detect.Target {
	return detect.Target{
		RelPath:    "target",
		Kind:       detect.KindDir,
		Reason:     "Cargo build output",
		Regenerate: "cargo build",
	}
}

type cargoFile struct {
	Package   *cargoPackage   `toml:"package"`
	Workspace *cargoWorkspace `toml:"workspace"`
}

type cargoPackage struct {
	Name string `toml:"name"`
}

type cargoWorkspace struct {
	Members []string `toml:"members"`
}

func shouldEmitRustTarget(ctx *detect.Context, dir string, cargo *cargoFile) bool {
	if cargo.Workspace != nil {
		return true
	}
	if cargo.Package == nil {
		return false
	}
	// Package-only crate: skip target if an ancestor workspace lists this dir as a member.
	return !isCargoWorkspaceMember(ctx, dir)
}

func isCargoWorkspaceMember(ctx *detect.Context, dir string) bool {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	cur := filepath.Dir(abs)
	for {
		manifest := filepath.Join(cur, "Cargo.toml")
		data, err := readFile(ctx, manifest)
		if err == nil {
			var cargo cargoFile
			if err := toml.Unmarshal(data, &cargo); err == nil && cargo.Workspace != nil {
				rel, err := filepath.Rel(cur, abs)
				if err == nil && memberMatch(cargo.Workspace.Members, filepath.ToSlash(rel)) {
					return true
				}
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return false
}

func memberMatch(members []string, rel string) bool {
	rel = strings.TrimPrefix(rel, "./")
	for _, m := range members {
		m = strings.TrimSpace(m)
		m = strings.TrimPrefix(m, "./")
		if m == "" {
			continue
		}
		ok, err := doublestar.Match(m, rel)
		if err == nil && ok {
			return true
		}
		// Also try **/member style when pattern has no slash.
		if !strings.Contains(m, "/") && !strings.ContainsAny(m, "*?[") {
			if rel == m || strings.HasSuffix(rel, "/"+m) {
				return true
			}
		}
	}
	return false
}

func readFile(ctx *detect.Context, path string) ([]byte, error) {
	if ctx != nil && ctx.Cache != nil {
		return ctx.Cache.ReadFile(path)
	}
	return os.ReadFile(path)
}
