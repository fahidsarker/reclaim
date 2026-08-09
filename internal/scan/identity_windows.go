//go:build windows

package scan

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

type fileID struct {
	volume uint32
	index  uint64
}

func fileIdentity(path string, info os.FileInfo) (fileID, error) {
	_ = info
	abs, err := filepath.Abs(path)
	if err != nil {
		return fileID{}, err
	}
	p, err := windows.UTF16PtrFromString(abs)
	if err != nil {
		return fileID{}, err
	}
	h, err := windows.CreateFile(
		p,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return fileID{}, err
	}
	defer windows.CloseHandle(h)

	var fi windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &fi); err != nil {
		return fileID{}, err
	}
	index := (uint64(fi.FileIndexHigh) << 32) | uint64(fi.FileIndexLow)
	return fileID{volume: fi.VolumeSerialNumber, index: index}, nil
}

func fileDevice(path string, info os.FileInfo) (uint64, error) {
	id, err := fileIdentity(path, info)
	if err != nil {
		return 0, err
	}
	return uint64(id.volume), nil
}
