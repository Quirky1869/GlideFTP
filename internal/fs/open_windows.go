//go:build windows

package fs

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

var _shellExecute = _shell32.NewProc("ShellExecuteW")

const (
	swShowNormal = 1
	seErrNoAssoc = 31 // SE_ERR_NOASSOC: no application associated with the file type
)

// OpenWithDefault opens path (file or folder) with its associated
// application via ShellExecuteW (verb "open"). With no association, it shows
// Windows' native "Open with..." chooser instead (OpenAs_RunDLL).
func OpenWithDefault(path string) error {
	verb, err := syscall.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	ret, _, _ := _shellExecute.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		0, 0,
		swShowNormal,
	)
	// ShellExecute returns a value > 32 on success.
	if ret > 32 {
		return nil
	}
	if ret == seErrNoAssoc {
		return exec.Command("rundll32.exe", "shell32.dll,OpenAs_RunDLL", path).Start()
	}
	return fmt.Errorf("ShellExecute failed (code %d)", ret)
}

// OpenWithApp launches appPath with filePath as its only argument.
func OpenWithApp(appPath, filePath string) error {
	return exec.Command(appPath, filePath).Start()
}
