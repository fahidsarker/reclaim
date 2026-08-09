package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

const controlFileName = ".reclaim.yaml"

// Mode is the control-file operating mode.
type Mode int

const (
	// ModeMerge runs detectors and adjusts with keep/delete (default).
	ModeMerge Mode = iota
	// ModeStrict ignores detectors; only delete: entries are candidates.
	ModeStrict
)

func (m Mode) String() string {
	switch m {
	case ModeStrict:
		return "strict"
	default:
		return "merge"
	}
}

// DeleteEntry is one delete: list item.
type DeleteEntry struct {
	Path       string
	Reason     string
	Regenerate string
}

// Control is a parsed .reclaim.yaml.
type Control struct {
	Dir               string // absolute directory containing the file
	Version           int
	Mode              Mode
	Inherit           bool
	Keep              []string
	Delete            []DeleteEntry
	FrameworksOnly    []string
	FrameworksDisable []string
	RequireGitIgnored bool
	Ignore            bool

	keepMatcher    *Matcher
	deleteMatcher  *Matcher
	deleteByPath   []deleteCompiled // parallel patterns for entry lookup
}

type deleteCompiled struct {
	entry   DeleteEntry
	matcher *Matcher // single-pattern matcher
}

type controlFileYAML struct {
	Version           *int           `yaml:"version"`
	Mode              string         `yaml:"mode"`
	Inherit           *bool          `yaml:"inherit"`
	Keep              []string       `yaml:"keep"`
	Delete            []rawDelete    `yaml:"delete"`
	Frameworks        *frameworksYAML `yaml:"frameworks"`
	RequireGitIgnored *bool          `yaml:"require_git_ignored"`
	Ignore            bool           `yaml:"ignore"`
}

type frameworksYAML struct {
	Only    []string `yaml:"only"`
	Disable []string `yaml:"disable"`
}

// rawDelete accepts a string or a mapping.
type rawDelete struct {
	Path       string
	Reason     string
	Regenerate string
}

func (r *rawDelete) UnmarshalYAML(b []byte) error {
	var s string
	if err := yaml.Unmarshal(b, &s); err == nil {
		r.Path = s
		return nil
	}
	var obj struct {
		Path       string `yaml:"path"`
		Reason     string `yaml:"reason"`
		Regenerate string `yaml:"regenerate"`
	}
	if err := yaml.Unmarshal(b, &obj); err != nil {
		return fmt.Errorf("delete entry: %w", err)
	}
	r.Path = obj.Path
	r.Reason = obj.Reason
	r.Regenerate = obj.Regenerate
	return nil
}

// LoadControl loads .reclaim.yaml from dir, or returns (nil, nil) if absent.
func LoadControl(dir string) (*Control, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	abs = filepath.Clean(abs)
	path := filepath.Join(abs, controlFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return ParseControl(abs, data)
}

// ParseControl parses and validates control file bytes for directory dir.
func ParseControl(dir string, data []byte) (*Control, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	abs = filepath.Clean(abs)

	var raw controlFileYAML
	if err := unmarshalYAML(data, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filepath.Join(abs, controlFileName), err)
	}

	c := &Control{
		Dir:               abs,
		Version:           1,
		Mode:              ModeMerge,
		Inherit:           true,
		RequireGitIgnored: true,
		Ignore:            raw.Ignore,
		Keep:              raw.Keep,
	}
	if raw.Version != nil {
		c.Version = *raw.Version
	}
	if c.Version != 1 {
		return nil, fmt.Errorf("%s: unsupported version %d (want 1)", controlFileName, c.Version)
	}
	switch strings.ToLower(strings.TrimSpace(raw.Mode)) {
	case "", "merge":
		c.Mode = ModeMerge
	case "strict":
		c.Mode = ModeStrict
	default:
		return nil, fmt.Errorf("%s: invalid mode %q (want merge or strict)", controlFileName, raw.Mode)
	}
	if raw.Inherit != nil {
		c.Inherit = *raw.Inherit
	}
	if raw.RequireGitIgnored != nil {
		c.RequireGitIgnored = *raw.RequireGitIgnored
	}
	if raw.Frameworks != nil {
		c.FrameworksOnly = raw.Frameworks.Only
		c.FrameworksDisable = raw.Frameworks.Disable
	}

	for _, d := range raw.Delete {
		path := strings.TrimSpace(d.Path)
		if path == "" {
			return nil, fmt.Errorf("%s: delete entry missing path", controlFileName)
		}
		reason := d.Reason
		if reason == "" {
			reason = "listed in .reclaim.yaml"
		}
		c.Delete = append(c.Delete, DeleteEntry{
			Path:       path,
			Reason:     reason,
			Regenerate: d.Regenerate,
		})
	}

	if err := c.compileMatchers(); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(abs, controlFileName), err)
	}
	return c, nil
}

func (c *Control) compileMatchers() error {
	km, err := CompileMatcher(c.Dir, c.Keep)
	if err != nil {
		return fmt.Errorf("keep: %w", err)
	}
	c.keepMatcher = km

	var delPatterns []string
	for _, d := range c.Delete {
		delPatterns = append(delPatterns, d.Path)
		sm, err := CompileMatcher(c.Dir, []string{d.Path})
		if err != nil {
			return fmt.Errorf("delete: %w", err)
		}
		c.deleteByPath = append(c.deleteByPath, deleteCompiled{entry: d, matcher: sm})
	}
	dm, err := CompileMatcher(c.Dir, delPatterns)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	c.deleteMatcher = dm
	return nil
}

// MatchKeep reports whether absPath matches a keep: pattern.
func (c *Control) MatchKeep(absPath string, isDir bool) bool {
	if c == nil {
		return false
	}
	return c.keepMatcher.Match(absPath, isDir)
}

// MatchDelete reports the delete entry matching absPath, if any.
func (c *Control) MatchDelete(absPath string, isDir bool) (DeleteEntry, bool) {
	if c == nil || c.deleteMatcher == nil || !c.deleteMatcher.Match(absPath, isDir) {
		return DeleteEntry{}, false
	}
	// Last matching non-negation entry provides reason/regenerate metadata.
	var found DeleteEntry
	ok := false
	for _, dc := range c.deleteByPath {
		if strings.HasPrefix(strings.TrimSpace(dc.entry.Path), "!") {
			continue
		}
		if dc.matcher.Match(absPath, isDir) {
			found = dc.entry
			ok = true
		}
	}
	if !ok {
		return DeleteEntry{Reason: "listed in .reclaim.yaml"}, true
	}
	return found, true
}

// FrameworkAllowed reports whether a detector may run under this control.
func (c *Control) FrameworkAllowed(name string) bool {
	if c == nil {
		return true
	}
	if len(c.FrameworksOnly) > 0 {
		ok := false
		for _, f := range c.FrameworksOnly {
			if f == name {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	for _, f := range c.FrameworksDisable {
		if f == name {
			return false
		}
	}
	return true
}

// Chain is the precedence stack of control files plus global config.
type Chain struct {
	Nearest   *Control   // local .reclaim.yaml for this directory, if any
	Ancestors []*Control // inherited controls, nearest ancestor first
	Global    *Global
}

// Effective returns the controlling file for mode/ignore/frameworks/require_git_ignored
// (nearest local, else first ancestor).
func (ch *Chain) Effective() *Control {
	if ch == nil {
		return nil
	}
	if ch.Nearest != nil {
		return ch.Nearest
	}
	if len(ch.Ancestors) > 0 {
		return ch.Ancestors[0]
	}
	return nil
}

// Ignore reports whether the subtree should be skipped entirely.
func (ch *Chain) Ignore() bool {
	eff := ch.Effective()
	return eff != nil && eff.Ignore
}

// Mode returns the effective mode (merge if none).
func (ch *Chain) Mode() Mode {
	eff := ch.Effective()
	if eff == nil {
		return ModeMerge
	}
	return eff.Mode
}

// RequireGitIgnored returns whether git vetoes apply (default true).
func (ch *Chain) RequireGitIgnored() bool {
	eff := ch.Effective()
	if eff == nil {
		return true
	}
	return eff.RequireGitIgnored
}

// FrameworkAllowed uses the effective control's framework filters.
func (ch *Chain) FrameworkAllowed(name string) bool {
	eff := ch.Effective()
	if eff == nil {
		return true
	}
	return eff.FrameworkAllowed(name)
}

// ForChildren returns the chain to pass to child directories.
// If the effective control has inherit: false, only global config is carried.
func (ch *Chain) ForChildren() *Chain {
	if ch == nil {
		return nil
	}
	eff := ch.Effective()
	if eff != nil && !eff.Inherit {
		if ch.Global == nil {
			return nil
		}
		return &Chain{Global: ch.Global}
	}
	return ch
}

// MatchKeep applies precedence: nearest keep, then ancestor keep, then global keep.
// A nearer delete does not produce keep; callers should use Classify.
func (ch *Chain) MatchKeep(absPath string, isDir bool) bool {
	keep, _ := ch.Classify(absPath, isDir)
	return keep
}

// Classify applies §5.2 keep/delete precedence (excluding hard safety and built-ins).
// keep=true means VerdictKept. del set means an explicit delete: match.
func (ch *Chain) Classify(absPath string, isDir bool) (keep bool, del *DeleteEntry) {
	if ch == nil {
		return false, nil
	}
	if ch.Nearest != nil {
		if ch.Nearest.MatchKeep(absPath, isDir) {
			return true, nil
		}
		if e, ok := ch.Nearest.MatchDelete(absPath, isDir); ok {
			return false, &e
		}
	}
	for _, a := range ch.Ancestors {
		if a.MatchKeep(absPath, isDir) {
			return true, nil
		}
		if e, ok := a.MatchDelete(absPath, isDir); ok {
			return false, &e
		}
	}
	if ch.Global != nil {
		if ch.Global.MatchKeep(absPath, isDir) {
			return true, nil
		}
		if e, ok := ch.Global.MatchDelete(absPath, isDir); ok {
			return false, &e
		}
	}
	return false, nil
}

// WithLocal returns a new chain with local as Nearest and prior controls as ancestors.
func (ch *Chain) WithLocal(local *Control) *Chain {
	out := &Chain{Nearest: local, Global: nil}
	if ch != nil {
		out.Global = ch.Global
		if ch.Nearest != nil {
			out.Ancestors = append(out.Ancestors, ch.Nearest)
		}
		out.Ancestors = append(out.Ancestors, ch.Ancestors...)
	}
	return out
}
