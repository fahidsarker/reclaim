//go:build windows

package exec

import "golang.org/x/sys/windows"

type fileID struct {
	volume uint32
	index  uint64
}

func identityFromPath(path string) (fileID, error) {
	p, err := windows.UTF16PtrFromString(path)
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

	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return fileID{}, err
	}
	index := (uint64(info.FileIndexHigh) << 32) | uint64(info.FileIndexLow)
	return fileID{volume: info.VolumeSerialNumber, index: index}, nil
}
