//go:build darwin

package exec

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
)

func platformTrash(path string, warn io.Writer) error {
	_ = warn
	// Use Finder via osascript — no cgo required.
	escaped := strings.ReplaceAll(path, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	script := fmt.Sprintf(`tell application "Finder" to delete (POSIX file "%s" as alias)`, escaped)
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("trash via osascript: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
