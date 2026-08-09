//go:build unix

package scan

import (
	"fmt"
	"os"
	"syscall"
)

type fileID struct {
	dev uint64
	ino uint64
}

func fileIdentity(path string, info os.FileInfo) (fileID, error) {
	_ = path
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileID{}, fmt.Errorf("unsupported stat type %T", info.Sys())
	}
	return fileID{dev: uint64(st.Dev), ino: uint64(st.Ino)}, nil
}

func fileDevice(path string, info os.FileInfo) (uint64, error) {
	id, err := fileIdentity(path, info)
	if err != nil {
		return 0, err
	}
	return id.dev, nil
}
