//go:build linux

package exec

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func platformTrash(path string, warn io.Writer) error {
	warn = ensureWarn(warn)

	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	trashHome, err := xdgTrashDir()
	if err != nil {
		return err
	}
	filesDir := filepath.Join(trashHome, "files")
	infoDir := filepath.Join(trashHome, "info")
	if err := os.MkdirAll(filesDir, 0700); err != nil {
		return err
	}
	if err := os.MkdirAll(infoDir, 0700); err != nil {
		return err
	}

	if same, err := sameDevice(abs, trashHome); err != nil {
		return err
	} else if !same {
		fmt.Fprintf(warn, "warning: %s is on a different filesystem than trash; permanently deleting\n", abs)
		return permanentlyRemove(abs)
	}

	base := filepath.Base(abs)
	destName := uniqueTrashName(filesDir, infoDir, base)
	dest := filepath.Join(filesDir, destName)
	infoPath := filepath.Join(infoDir, destName+".trashinfo")

	info := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		escapeTrashPath(abs),
		time.Now().Format("2006-01-02T15:04:05"),
	)
	if err := os.WriteFile(infoPath, []byte(info), 0600); err != nil {
		return err
	}
	if err := os.Rename(abs, dest); err != nil {
		_ = os.Remove(infoPath)
		return err
	}
	return nil
}

func xdgTrashDir() (string, error) {
	if data := os.Getenv("XDG_DATA_HOME"); data != "" {
		return filepath.Join(data, "Trash"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "Trash"), nil
}

func sameDevice(a, b string) (bool, error) {
	var sa, sb syscall.Stat_t
	if err := syscall.Stat(a, &sa); err != nil {
		return false, err
	}
	if err := syscall.Stat(b, &sb); err != nil {
		return false, err
	}
	return sa.Dev == sb.Dev, nil
}

func uniqueTrashName(filesDir, infoDir, base string) string {
	candidate := base
	for i := 1; ; i++ {
		if _, err := os.Lstat(filepath.Join(filesDir, candidate)); err == nil {
			candidate = fmt.Sprintf("%s.%d", base, i)
			continue
		}
		if _, err := os.Lstat(filepath.Join(infoDir, candidate+".trashinfo")); err == nil {
			candidate = fmt.Sprintf("%s.%d", base, i)
			continue
		}
		return candidate
	}
}

func escapeTrashPath(path string) string {
	var b strings.Builder
	for _, r := range path {
		if r <= 0x20 || r == 0x7f || r == '%' {
			fmt.Fprintf(&b, "%%%02X", r)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
