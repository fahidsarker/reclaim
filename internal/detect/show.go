package detect

import (
	"fmt"
	"sort"

	"github.com/goccy/go-yaml"
)

// FindDetector returns the registered detector with the given name.
func FindDetector(name string) Detector {
	for _, d := range Detectors() {
		if d.Name() == name {
			return d
		}
	}
	return nil
}

// FindResolvedSpec returns the resolved YAML spec for name, if any.
func FindResolvedSpec(name string) *ResolvedSpec {
	if d := FindDetector(name); d != nil {
		if sd, ok := d.(*SpecDetector); ok {
			return sd.Spec
		}
	}
	for _, s := range EmbeddedSpecs() {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// FormatResolvedSpecYAML dumps a resolved spec after extends expansion.
func FormatResolvedSpecYAML(rs *ResolvedSpec) (string, error) {
	if rs == nil {
		return "", fmt.Errorf("nil spec")
	}
	type dumpTarget struct {
		Path       string `yaml:"path"`
		Reason     string `yaml:"reason,omitempty"`
		Regenerate string `yaml:"regenerate,omitempty"`
		Safety     string `yaml:"safety,omitempty"`
	}
	type dump struct {
		Name        string         `yaml:"name"`
		Description string         `yaml:"description,omitempty"`
		Priority    int            `yaml:"priority"`
		Detect      Predicate      `yaml:"detect"`
		Targets     []dumpTarget   `yaml:"targets"`
		Metadata    map[string]any `yaml:"metadata,omitempty"`
	}
	out := dump{
		Name:        rs.Name,
		Description: rs.Description,
		Priority:    rs.Priority,
		Detect:      rs.Detect,
		Metadata:    rs.Metadata,
	}
	for _, t := range rs.Targets {
		out.Targets = append(out.Targets, dumpTarget{
			Path:       t.Path,
			Reason:     t.Reason,
			Regenerate: t.Regenerate,
			Safety:     t.Safety,
		})
	}
	data, err := yaml.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DetectorDescription returns a short description when available.
func DetectorDescription(d Detector) string {
	if sd, ok := d.(*SpecDetector); ok && sd.Spec != nil {
		return sd.Spec.Description
	}
	switch d.Name() {
	case "rust":
		return "Rust / Cargo projects (workspace-aware)"
	case "bazel":
		return "Bazel workspaces"
	default:
		return ""
	}
}

// ListDetectorsSorted returns detectors sorted by name.
func ListDetectorsSorted() []Detector {
	list := Detectors()
	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
	return list
}
