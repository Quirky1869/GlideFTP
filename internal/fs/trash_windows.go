//go:build windows

package fs

import (
	"fmt"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	_shell32        = syscall.NewLazyDLL("shell32.dll")
	_shFileOperation = _shell32.NewProc("SHFileOperationW")
)

// shFileOpStruct mirrors SHFILEOPSTRUCTW on 64-bit Windows (56 bytes).
// Padding fields match MSVC's layout exactly.
type shFileOpStruct struct {
	hwnd     uintptr // HWND   (8 bytes)
	wFunc    uint32  // UINT   (4 bytes)
	_pad1    uint32  // padding to 8-byte boundary for pFrom
	pFrom    uintptr // LPCWSTR (8 bytes)
	pTo      uintptr // LPCWSTR (8 bytes)
	fFlags   uint16  // FILEOP_FLAGS (2 bytes)
	_pad2    uint16  // padding to 4-byte boundary for fAborted
	fAborted int32   // BOOL (4 bytes)
	hMapping uintptr // LPVOID (8 bytes)
	title    uintptr // LPCWSTR (8 bytes)
}

const (
	foDelete          uint32 = 0x0003
	fofAllowUndo      uint16 = 0x0040
	fofNoConfirmation uint16 = 0x0010
	fofSilent         uint16 = 0x0004
	fofNoErrorUI      uint16 = 0x0400
)

// Trash moves path to the Windows Recycle Bin using SHFileOperationW.
func Trash(path string) error {
	// SHFileOperationW requires a double-null terminated UTF-16 multi-string.
	runes := []rune(path)
	runes = append(runes, 0, 0) // two null runes → double-null in UTF-16
	p := utf16.Encode(runes)

	op := shFileOpStruct{
		wFunc:  foDelete,
		pFrom:  uintptr(unsafe.Pointer(&p[0])),
		fFlags: fofAllowUndo | fofNoConfirmation | fofSilent | fofNoErrorUI,
	}

	ret, _, _ := _shFileOperation.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return fmt.Errorf("failed to move to recycle bin (code %d)", ret)
	}
	return nil
}
