//go:build windows

package searchindex

import (
	"os"
	"syscall"
)

// openLocked opens (creating it if needed) path with share mode 0: while
// GlideFTP keeps it open, Windows refuses any other program reading,
// writing, deleting or renaming it - and renaming its folder. Explorer then
// shows its own "The action can't be completed because the file is open in
// GlideFTP" message. (os.OpenFile always shares read/write, hence CreateFile.)
func openLocked(path string) (*os.File, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(p,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0, // no sharing
		nil,
		syscall.OPEN_ALWAYS,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(h), path), nil
}
