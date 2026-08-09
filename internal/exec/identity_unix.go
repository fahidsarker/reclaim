//go:build unix

package exec

import (
	"fmt"
	"os"
	"syscall"
)

type fileID struct {
	dev uint64
	ino uint64
}

func identityFromPath(path string) (fileID, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return fileID{}, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileID{}, fmt.Errorf("unsupported stat type %T", info.Sys())
	}
	return fileID{dev: uint64(st.Dev), ino: uint64(st.Ino)}, nil
}
