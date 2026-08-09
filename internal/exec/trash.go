package exec

import (
	"fmt"
	"io"
)

// TrashFunc moves a path to the OS trash/recycle bin.
// warn may be used for non-fatal notices (e.g. cross-device fallback).
type TrashFunc func(path string, warn io.Writer) error

func defaultTrash() TrashFunc {
	return platformTrash
}

// trashOrRemove sends path to trash, or permanently removes it when ToTrash is false.
func trashOrRemove(path string, toTrash bool, trash TrashFunc, warn io.Writer) error {
	if !toTrash {
		return removeAll(path)
	}
	if trash == nil {
		trash = defaultTrash()
	}
	if trash == nil {
		return fmt.Errorf("trash is not supported on this platform")
	}
	return trash(path, warn)
}

// ensureWarn returns w or Discard.
func ensureWarn(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}

// permanentlyRemove is used by trash backends as a fallback.
func permanentlyRemove(path string) error {
	return removeAll(path)
}
