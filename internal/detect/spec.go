package detect

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"
)

// SpecFile is the on-disk / embedded YAML representation of a framework spec.
type SpecFile struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Priority    int               `yaml:"priority"`
	Extends     string            `yaml:"extends"`
	Detect      Predicate         `yaml:"detect"`
	Targets     []SpecTarget      `yaml:"targets"`
	Metadata    map[string]any    `yaml:"metadata"`
}

// SpecTarget is a declarative deletion candidate.
type SpecTarget struct {
	Path       string     `yaml:"path"`
	Reason     string     `yaml:"reason"`
	Regenerate string     `yaml:"regenerate"`
	Safety     string     `yaml:"safety"` // "" or "requires_flag"
	When       *Predicate `yaml:"when"`
}

// ResolvedSpec is a fully expanded spec ready for detection.
type ResolvedSpec struct {
	Name        string
	Description string
	Priority    int
	Detect      Predicate
	Targets     []SpecTarget
	Metadata    map[string]any
	ManifestHint string // first file_exists / path file in detect tree, for Match.Manifest
}

// SpecDetector evaluates a ResolvedSpec against a directory.
type SpecDetector struct {
	Spec *ResolvedSpec
}

func (d *SpecDetector) Name() string  { return d.Spec.Name }
func (d *SpecDetector) Priority() int { return d.Spec.Priority }

func (d *SpecDetector) Detect(ctx *Context, dir string) (*Match, error) {
	switch d.Spec.Detect.eval(ctx, dir) {
	case predPass:
		targets, err := d.evalTargets(ctx, dir)
		if err != nil {
			return nil, err
		}
		return &Match{
			Framework:  d.Spec.Name,
			Confidence: ConfidenceStrong,
			Manifest:   resolveManifest(ctx, dir, d.Spec),
			Targets:    targets,
			Metadata:   evalMetadata(ctx, dir, d.Spec.Metadata),
		}, nil
	case predParse:
		targets, err := d.evalTargets(ctx, dir)
		if err != nil {
			return nil, err
		}
		// Weak: expose would-be targets so the walker can skip them loudly.
		return &Match{
			Framework:  d.Spec.Name,
			Confidence: ConfidenceWeak,
			Manifest:   resolveManifest(ctx, dir, d.Spec),
			Targets:    targets,
			Metadata:   nil,
		}, nil
	default:
		return nil, nil
	}
}

func (d *SpecDetector) evalTargets(ctx *Context, dir string) ([]Target, error) {
	var out []Target
	for _, st := range d.Spec.Targets {
		if st.When != nil {
			switch st.When.eval(ctx, dir) {
			case predPass:
				// include
			default:
				continue
			}
		}
		kind := KindDir
		rel := filepath.FromSlash(st.Path)
		if isGlobPath(st.Path) {
			kind = KindGlob
		}
		safety := SafetyNormal
		if st.Safety == "requires_flag" {
			safety = SafetyRequiresFlag
		}
		out = append(out, Target{
			RelPath:    rel,
			Kind:       kind,
			Reason:     st.Reason,
			Regenerate: st.Regenerate,
			Safety:     safety,
		})
	}
	return out, nil
}

func isGlobPath(p string) bool {
	return strings.ContainsAny(p, "*?[") || strings.Contains(p, "**")
}

func resolveManifest(ctx *Context, dir string, spec *ResolvedSpec) string {
	if spec.ManifestHint != "" {
		p := filepath.Join(dir, spec.ManifestHint)
		if _, err := lstat(ctx, p); err == nil {
			return p
		}
	}
	return dir
}

func evalMetadata(ctx *Context, dir string, meta map[string]any) map[string]string {
	if len(meta) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, raw := range meta {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		ffe, ok := m["from_file_exists"].(map[string]any)
		if !ok {
			// goccy may decode nested maps as map[string]string already via yaml
			if ffs, ok := m["from_file_exists"].(map[string]string); ok {
				for file, val := range ffs {
					if existsFile(ctx, filepath.Join(dir, file)) == predPass {
						out[key] = val
						break
					}
				}
			}
			continue
		}
		for file, val := range ffe {
			s, _ := val.(string)
			if s == "" {
				continue
			}
			if existsFile(ctx, filepath.Join(dir, file)) == predPass {
				out[key] = s
				break
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ParseSpecYAML parses and validates a single spec document (before extends expansion).
func ParseSpecYAML(data []byte) (*SpecFile, error) {
	var sf SpecFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}
	if err := validateSpecFile(&sf); err != nil {
		return nil, err
	}
	return &sf, nil
}

func validateSpecFile(sf *SpecFile) error {
	if sf.Name == "" {
		return fmt.Errorf("spec: name is required")
	}
	if err := sf.Detect.validate("detect"); err != nil {
		return fmt.Errorf("spec %s: %w", sf.Name, err)
	}
	for i, t := range sf.Targets {
		if t.Path == "" {
			return fmt.Errorf("spec %s: targets[%d].path is required", sf.Name, i)
		}
		if t.Safety != "" && t.Safety != "requires_flag" {
			return fmt.Errorf("spec %s: targets[%d].safety must be empty or requires_flag", sf.Name, i)
		}
		if t.When != nil {
			if err := t.When.validate(fmt.Sprintf("targets[%d].when", i)); err != nil {
				return fmt.Errorf("spec %s: %w", sf.Name, err)
			}
		}
	}
	return nil
}

// LoadSpecsFromFS reads *.yaml specs from fsys, expands extends, and returns resolved specs.
func LoadSpecsFromFS(fsys fs.FS) ([]*ResolvedSpec, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	byName := map[string]*SpecFile{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		sf, err := ParseSpecYAML(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if _, dup := byName[sf.Name]; dup {
			return nil, fmt.Errorf("duplicate spec name %q", sf.Name)
		}
		byName[sf.Name] = sf
	}
	if len(byName) == 0 {
		return nil, fmt.Errorf("no YAML specs found")
	}

	var resolved []*ResolvedSpec
	for name := range byName {
		rs, err := expandExtends(name, byName, nil)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, rs)
	}
	return resolved, nil
}

func expandExtends(name string, byName map[string]*SpecFile, stack []string) (*ResolvedSpec, error) {
	for _, s := range stack {
		if s == name {
			return nil, fmt.Errorf("extends cycle involving %q", name)
		}
	}
	sf, ok := byName[name]
	if !ok {
		return nil, fmt.Errorf("unknown extends target %q", name)
	}
	stack = append(stack, name)

	var parent *ResolvedSpec
	if sf.Extends != "" {
		var err error
		parent, err = expandExtends(sf.Extends, byName, stack)
		if err != nil {
			return nil, err
		}
	}

	rs := &ResolvedSpec{
		Name:        sf.Name,
		Description: sf.Description,
		Priority:    sf.Priority,
		Metadata:    sf.Metadata,
	}
	if parent != nil {
		rs.Detect = Predicate{All: []Predicate{parent.Detect, sf.Detect}}
		rs.Targets = mergeTargets(parent.Targets, sf.Targets)
		if rs.Priority == 0 {
			rs.Priority = parent.Priority
		}
		if rs.Description == "" {
			rs.Description = parent.Description
		}
		rs.ManifestHint = parent.ManifestHint
		if rs.Metadata == nil {
			rs.Metadata = parent.Metadata
		}
	} else {
		rs.Detect = sf.Detect
		rs.Targets = append([]SpecTarget(nil), sf.Targets...)
	}
	if hint := firstManifestHint(&sf.Detect); hint != "" {
		rs.ManifestHint = hint
	}
	if rs.Priority == 0 {
		rs.Priority = 10
	}
	return rs, nil
}

func mergeTargets(parent, child []SpecTarget) []SpecTarget {
	seen := map[string]int{}
	out := make([]SpecTarget, 0, len(parent)+len(child))
	for _, t := range parent {
		seen[t.Path] = len(out)
		out = append(out, t)
	}
	for _, t := range child {
		if i, ok := seen[t.Path]; ok {
			out[i] = t
			continue
		}
		seen[t.Path] = len(out)
		out = append(out, t)
	}
	return out
}

func firstManifestHint(p *Predicate) string {
	if p == nil {
		return ""
	}
	switch {
	case p.FileExists != "":
		return p.FileExists
	case p.JSONPath != nil:
		return p.JSONPath.File
	case p.YAMLPath != nil:
		return p.YAMLPath.File
	case p.TOMLPath != nil:
		return p.TOMLPath.File
	case p.FileContains != nil:
		return p.FileContains.File
	case len(p.All) > 0:
		for i := range p.All {
			if h := firstManifestHint(&p.All[i]); h != "" {
				return h
			}
		}
	case len(p.Any) > 0:
		for i := range p.Any {
			if h := firstManifestHint(&p.Any[i]); h != "" {
				return h
			}
		}
	case p.Not != nil:
		return firstManifestHint(p.Not)
	}
	return ""
}

// TargetPruneBasenames returns first path segments of all literal (non-glob) targets
// across resolved specs, used to seed walker prune/orphan detection.
func TargetPruneBasenames(specs []*ResolvedSpec) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range specs {
		for _, t := range s.Targets {
			if isGlobPath(t.Path) {
				// For globs like **/__pycache__ or *.egg-info, also prune the basename pattern tip.
				base := path.Base(t.Path)
				if base != "" && base != "." && base != "*" && !strings.ContainsAny(base, "*?[") {
					if _, ok := seen[base]; !ok {
						seen[base] = struct{}{}
						out = append(out, base)
					}
				}
				if base == "__pycache__" || strings.HasSuffix(t.Path, "__pycache__") {
					if _, ok := seen["__pycache__"]; !ok {
						seen["__pycache__"] = struct{}{}
						out = append(out, "__pycache__")
					}
				}
				continue
			}
			rel := filepath.ToSlash(t.Path)
			seg := strings.SplitN(rel, "/", 2)[0]
			if seg == "" || seg == "." {
				continue
			}
			if _, ok := seen[seg]; ok {
				continue
			}
			seen[seg] = struct{}{}
			out = append(out, seg)
		}
	}
	return out
}

var (
	embeddedOnce   sync.Once
	embeddedErr    error
	embeddedSpecs  []*ResolvedSpec
	pruneBasenames []string
)

// MustLoadEmbedded parses embedded specs and registers SpecDetectors. Safe to call multiple times.
func MustLoadEmbedded() {
	embeddedOnce.Do(func() {
		sub, err := fs.Sub(specsFS, "specs")
		if err != nil {
			embeddedErr = fmt.Errorf("specs embed: %w", err)
			return
		}
		specs, err := LoadSpecsFromFS(sub)
		if err != nil {
			embeddedErr = err
			return
		}
		embeddedSpecs = specs
		pruneBasenames = TargetPruneBasenames(specs)
		for _, s := range specs {
			Register(&SpecDetector{Spec: s})
		}
	})
	if embeddedErr != nil {
		panic(embeddedErr)
	}
}

// EmbeddedSpecs returns the resolved embedded specs (after MustLoadEmbedded).
func EmbeddedSpecs() []*ResolvedSpec {
	MustLoadEmbedded()
	return embeddedSpecs
}

// PruneBasenames returns artifact basename seeds from embedded specs.
func PruneBasenames() []string {
	MustLoadEmbedded()
	out := make([]string, len(pruneBasenames))
	copy(out, pruneBasenames)
	return out
}

// ResetRegistryForTest clears registered detectors. For tests only.
func ResetRegistryForTest() {
	registryMu.Lock()
	defer registryMu.Unlock()
	detectors = nil
	embeddedOnce = sync.Once{}
	embeddedErr = nil
	embeddedSpecs = nil
	pruneBasenames = nil
}
