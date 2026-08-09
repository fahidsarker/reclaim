package detect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadUserSpecsFromDir loads *.yaml / *.yml specs from dir, registers valid ones,
// and appends their prune basenames. Malformed specs are skipped via warn.
// Missing dir is a no-op. Extends may reference embedded specs by name.
// Specs whose name is already registered are warned and skipped.
func LoadUserSpecsFromDir(dir string, warn func(string)) int {
	if dir == "" {
		return 0
	}
	if warn == nil {
		warn = func(string) {}
	}
	MustLoadEmbedded()

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		warn(fmt.Sprintf("user specs: cannot read %s: %v", dir, err))
		return 0
	}

	byName := map[string]*SpecFile{}
	var order []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			warn(fmt.Sprintf("user specs: skip %s: %v", name, err))
			continue
		}
		sf, err := ParseSpecYAML(data)
		if err != nil {
			warn(fmt.Sprintf("user specs: skip %s: %v", name, err))
			continue
		}
		if _, dup := byName[sf.Name]; dup {
			warn(fmt.Sprintf("user specs: skip %s: duplicate name %q", name, sf.Name))
			continue
		}
		byName[sf.Name] = sf
		order = append(order, sf.Name)
	}

	var registered int
	var resolved []*ResolvedSpec
	for _, name := range order {
		if nameRegistered(name) {
			warn(fmt.Sprintf("user specs: skip %q: detector already registered", name))
			continue
		}
		rs, err := expandUserExtends(name, byName, nil)
		if err != nil {
			warn(fmt.Sprintf("user specs: skip %q: %v", name, err))
			continue
		}
		Register(&SpecDetector{Spec: rs, Source: SourceUser})
		resolved = append(resolved, rs)
		registered++
	}
	if len(resolved) > 0 {
		AppendPruneBasenames(TargetPruneBasenames(resolved))
	}
	return registered
}

func nameRegistered(name string) bool {
	for _, d := range Detectors() {
		if d.Name() == name {
			return true
		}
	}
	return false
}

func expandUserExtends(name string, userByName map[string]*SpecFile, stack []string) (*ResolvedSpec, error) {
	for _, s := range stack {
		if s == name {
			return nil, fmt.Errorf("extends cycle involving %q", name)
		}
	}
	sf, ok := userByName[name]
	if !ok {
		if emb := findEmbeddedResolved(name); emb != nil {
			return emb, nil
		}
		return nil, fmt.Errorf("unknown extends target %q", name)
	}
	stack = append(stack, name)

	var parent *ResolvedSpec
	if sf.Extends != "" {
		var err error
		parent, err = expandUserExtends(sf.Extends, userByName, stack)
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

func findEmbeddedResolved(name string) *ResolvedSpec {
	for _, s := range EmbeddedSpecs() {
		if s.Name == name {
			return s
		}
	}
	return nil
}
