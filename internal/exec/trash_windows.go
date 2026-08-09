//go:build windows

package exec

import (
	"fmt"
	"io"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	foDelete          = 0x0003
	fofAllowUndo      = 0x0040
	fofNoConfirmation = 0x0010
	fofSilent         = 0x0004
	fofNoErrorUI      = 0x0400
)

// shFileOpStruct matches SHFILEOPSTRUCTW.
type shFileOpStruct struct {
	hwnd                  windows.Handle
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

var (
	modShell32           = windows.NewLazySystemDLL("shell32.dll")
	procSHFileOperationW = modShell32.NewProc("SHFileOperationW")
)

func platformTrash(path string, warn io.Writer) error {
	_ = warn
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	// Double-null-terminated path list.
	from, err := windows.UTF16FromString(abs)
	if err != nil {
		return err
	}
	from = append(from, 0)

	op := shFileOpStruct{
		wFunc:  foDelete,
		pFrom:  &from[0],
		fFlags: fofAllowUndo | fofNoConfirmation | fofSilent | fofNoErrorUI,
	}
	r, _, callErr := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if r != 0 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return fmt.Errorf("SHFileOperationW: %w (code %d)", callErr, r)
		}
		return fmt.Errorf("SHFileOperationW failed with code %d", r)
	}
	if op.fAnyOperationsAborted != 0 {
		return fmt.Errorf("SHFileOperationW aborted")
	}
	return nil
}
