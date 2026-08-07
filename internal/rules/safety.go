package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var vcsDirNames = map[string]struct{}{
	".git": {},
	".hg":  {},
	".svn": {},
	".jj":  {},
}

// IsVCSDir reports whether basename is a hard-pruned VCS directory.
func IsVCSDir(name string) bool {
	_, ok := vcsDirNames[name]
	return ok
}

// GuardRoot aborts if root is /, $HOME, or a filesystem root without acknowledgement.
func GuardRoot(root string, iKnowWhatImDoing bool) error {
	if iKnowWhatImDoing {
		return nil
	}

	abs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve scan root: %w", err)
	}
	abs = filepath.Clean(abs)

	if abs == string(filepath.Separator) || abs == "/" {
		return fmt.Errorf("refusing to scan filesystem root %q without --i-know-what-im-doing", abs)
	}

	home, err := os.UserHomeDir()
	if err == nil {
		home = filepath.Clean(home)
		if abs == home {
			return fmt.Errorf("refusing to scan home directory %q without --i-know-what-im-doing", abs)
		}
	}

	// Volume root on Windows (e.g. C:\).
	if runtime.GOOS == "windows" {
		vol := filepath.VolumeName(abs)
		if vol != "" && (abs == vol+`\` || abs == vol+`/`) {
			return fmt.Errorf("refusing to scan filesystem root %q without --i-know-what-im-doing", abs)
		}
	}

	return nil
}

// CheckTarget validates a candidate target against the hard denylist.
// Returns a human-readable reason if the target must be skipped/dropped, or "" if safe.
func CheckTarget(scanRoot, targetPath string) string {
	abs, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Sprintf("cannot resolve path: %v", err)
	}
	abs = filepath.Clean(abs)

	rootAbs, err := filepath.Abs(scanRoot)
	if err != nil {
		return fmt.Sprintf("cannot resolve scan root: %v", err)
	}
	rootAbs = filepath.Clean(rootAbs)

	if reason := checkForbiddenPath(abs); reason != "" {
		return reason
	}

	if reason := checkVCSInPath(abs); reason != "" {
		return reason
	}

	if reason := checkStateDenylist(filepath.Base(abs)); reason != "" {
		return reason
	}

	// Resolve root through symlinks too (e.g. macOS /var → /private/var).
	rootResolved := rootAbs
	if r, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootResolved = filepath.Clean(r)
	}

	if !pathWithinRoot(abs, rootAbs) && !pathWithinRoot(abs, rootResolved) {
		return "path escapes scan root"
	}

	// Symlink escape / path containment after EvalSymlinks.
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// Path may not exist; logical containment already checked.
		return ""
	}
	resolved = filepath.Clean(resolved)

	if reason := checkForbiddenPath(resolved); reason != "" {
		return reason
	}
	if reason := checkVCSInPath(resolved); reason != "" {
		return reason
	}
	if !pathWithinRoot(resolved, rootResolved) {
		return "path escapes scan root (symlink)"
	}

	return ""
}

func checkForbiddenPath(abs string) string {
	sep := string(filepath.Separator)
	if abs == sep || abs == "/" {
		return "target is filesystem root"
	}

	home, err := os.UserHomeDir()
	if err == nil {
		home = filepath.Clean(home)
		forbidden := []string{
			home,
			filepath.Join(home, "Desktop"),
			filepath.Join(home, "Documents"),
			filepath.Join(home, "Downloads"),
		}
		for _, f := range forbidden {
			if abs == filepath.Clean(f) {
				return fmt.Sprintf("target is protected path %q", f)
			}
		}
		for _, env := range []string{"XDG_DATA_HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"} {
			if v := os.Getenv(env); v != "" {
				v = filepath.Clean(v)
				if abs == v {
					return fmt.Sprintf("target is XDG base directory %q", v)
				}
			}
		}
	}
	return ""
}

func checkVCSInPath(abs string) string {
	parts := strings.Split(abs, string(filepath.Separator))
	for _, p := range parts {
		if IsVCSDir(p) {
			return fmt.Sprintf("target is or contains VCS directory %q", p)
		}
	}
	return ""
}

func checkStateDenylist(base string) string {
	lower := strings.ToLower(base)
	patterns := []struct {
		match func(string) bool
		label string
	}{
		{func(s string) bool { return strings.HasPrefix(s, "terraform.tfstate") }, "terraform.tfstate*"},
		{func(s string) bool { return strings.HasSuffix(s, ".sqlite") }, "*.sqlite"},
		{func(s string) bool { return strings.HasSuffix(s, ".db") }, "*.db"},
		{func(s string) bool { return strings.HasPrefix(s, ".env") }, ".env*"},
		{func(s string) bool { return strings.HasSuffix(s, ".pem") }, "*.pem"},
		{func(s string) bool { return strings.HasSuffix(s, ".key") }, "*.key"},
		{func(s string) bool { return strings.HasPrefix(s, "id_rsa") }, "id_rsa*"},
	}
	for _, p := range patterns {
		if p.match(lower) || p.match(base) {
			return fmt.Sprintf("basename matches state denylist (%s)", p.label)
		}
	}
	return ""
}

func pathWithinRoot(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
