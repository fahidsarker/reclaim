package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Global is the lowest-precedence user config (~/.config/reclaim/config.yaml).
type Global struct {
	Keep   []string
	Delete []DeleteEntry

	base           string // directory used as pattern base (scan root at apply time)
	keepMatcher    *Matcher
	deleteByPath   []deleteCompiled
	deleteMatcher  *Matcher
}

type globalFileYAML struct {
	Keep   []string    `yaml:"keep"`
	Delete []rawDelete `yaml:"delete"`
}

// DefaultGlobalPath returns ~/.config/reclaim/config.yaml (or OS equivalent).
func DefaultGlobalPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "reclaim", "config.yaml"), nil
}

// DefaultSpecsDir returns ~/.config/reclaim/specs (or OS equivalent).
func DefaultSpecsDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "reclaim", "specs"), nil
}

// LoadGlobal loads the global config file. Missing file yields an empty Global.
// Patterns are compiled relative to patternBase (typically the scan root).
func LoadGlobal(patternBase string) (*Global, error) {
	path, err := DefaultGlobalPath()
	if err != nil {
		return emptyGlobal(patternBase)
	}
	return LoadGlobalFile(path, patternBase)
}

// LoadGlobalFile loads global config from path. Missing file yields empty Global.
func LoadGlobalFile(path, patternBase string) (*Global, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyGlobal(patternBase)
		}
		return nil, err
	}
	return ParseGlobal(patternBase, data)
}

func emptyGlobal(patternBase string) (*Global, error) {
	g := &Global{}
	if err := g.compile(patternBase); err != nil {
		return nil, err
	}
	return g, nil
}

// ParseGlobal parses global config bytes; patterns are relative to patternBase.
func ParseGlobal(patternBase string, data []byte) (*Global, error) {
	var raw globalFileYAML
	if err := unmarshalYAML(data, &raw); err != nil {
		return nil, fmt.Errorf("parse global config: %w", err)
	}
	g := &Global{Keep: raw.Keep}
	for _, d := range raw.Delete {
		path := strings.TrimSpace(d.Path)
		if path == "" {
			return nil, fmt.Errorf("global config: delete entry missing path")
		}
		reason := d.Reason
		if reason == "" {
			reason = "listed in global config"
		}
		g.Delete = append(g.Delete, DeleteEntry{
			Path:       path,
			Reason:     reason,
			Regenerate: d.Regenerate,
		})
	}
	if err := g.compile(patternBase); err != nil {
		return nil, err
	}
	return g, nil
}

func (g *Global) compile(patternBase string) error {
	abs, err := filepath.Abs(patternBase)
	if err != nil {
		return err
	}
	g.base = filepath.Clean(abs)
	km, err := CompileMatcher(g.base, g.Keep)
	if err != nil {
		return fmt.Errorf("global keep: %w", err)
	}
	g.keepMatcher = km

	var delPatterns []string
	g.deleteByPath = nil
	for _, d := range g.Delete {
		delPatterns = append(delPatterns, d.Path)
		sm, err := CompileMatcher(g.base, []string{d.Path})
		if err != nil {
			return fmt.Errorf("global delete: %w", err)
		}
		g.deleteByPath = append(g.deleteByPath, deleteCompiled{entry: d, matcher: sm})
	}
	dm, err := CompileMatcher(g.base, delPatterns)
	if err != nil {
		return fmt.Errorf("global delete: %w", err)
	}
	g.deleteMatcher = dm
	return nil
}

// MatchKeep reports whether absPath matches a global keep: pattern.
func (g *Global) MatchKeep(absPath string, isDir bool) bool {
	if g == nil || g.keepMatcher == nil {
		return false
	}
	return g.keepMatcher.Match(absPath, isDir)
}

// MatchDelete reports the global delete entry matching absPath, if any.
func (g *Global) MatchDelete(absPath string, isDir bool) (DeleteEntry, bool) {
	if g == nil || g.deleteMatcher == nil || !g.deleteMatcher.Match(absPath, isDir) {
		return DeleteEntry{}, false
	}
	var found DeleteEntry
	ok := false
	for _, dc := range g.deleteByPath {
		if strings.HasPrefix(strings.TrimSpace(dc.entry.Path), "!") {
			continue
		}
		if dc.matcher.Match(absPath, isDir) {
			found = dc.entry
			ok = true
		}
	}
	if !ok {
		return DeleteEntry{Reason: "listed in global config"}, true
	}
	return found, true
}
