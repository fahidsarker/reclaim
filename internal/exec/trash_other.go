//go:build !darwin && !linux && !windows

package exec

import (
	"fmt"
	"io"
	"runtime"
)

func platformTrash(path string, warn io.Writer) error {
	_ = path
	_ = warn
	return fmt.Errorf("trash is not supported on %s", runtime.GOOS)
}
