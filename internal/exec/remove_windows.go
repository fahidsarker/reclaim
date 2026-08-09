//go:build windows

package exec

import (
	"io/fs"
	"os"
	"path/filepath"
)

func removeAll(path string) error {
	err := os.RemoveAll(path)
	if err == nil {
		return nil
	}
	// Retry after clearing read-only attributes (common on Windows).
	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Mode()&0200 == 0 {
			_ = os.Chmod(p, info.Mode()|0200)
		}
		return nil
	})
	return os.RemoveAll(path)
}
