//go:build linux

package fs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Trash moves path to the XDG Trash directory (~/.local/share/Trash/).
// Falls back to a cross-device copy+delete if the file is on a different
// filesystem than the trash directory.
func Trash(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	trashBase := xdgTrashDir()
	filesDir := filepath.Join(trashBase, "files")
	infoDir := filepath.Join(trashBase, "info")
	if err := os.MkdirAll(filesDir, 0700); err != nil {
		return err
	}
	if err := os.MkdirAll(infoDir, 0700); err != nil {
		return err
	}

	// Resolve a unique name inside the trash (handle collisions).
	name := filepath.Base(absPath)
	destPath, infoPath := trashPaths(filesDir, infoDir, name)

	// Move the file; if cross-device, copy then delete.
	if err := os.Rename(absPath, destPath); err != nil {
		var errno syscall.Errno
		if errors.As(err, &errno) && errno == syscall.EXDEV {
			if copyErr := Copy(absPath, destPath); copyErr != nil {
				return fmt.Errorf("trash (cross-device copy): %w", copyErr)
			}
			if rmErr := os.RemoveAll(absPath); rmErr != nil {
				_ = os.RemoveAll(destPath)
				return fmt.Errorf("trash (cross-device delete): %w", rmErr)
			}
		} else {
			return err
		}
	}

	// Write the .trashinfo sidecar.
	info := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		absPath, time.Now().Format("2006-01-02T15:04:05"))
	return os.WriteFile(infoPath, []byte(info), 0600)
}

func xdgTrashDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "Trash")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "Trash")
}

// trashPaths returns unique files/ and info/ paths for a given base name.
func trashPaths(filesDir, infoDir, name string) (destPath, infoPath string) {
	destPath = filepath.Join(filesDir, name)
	infoPath = filepath.Join(infoDir, name+".trashinfo")
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return
	}
	ext := filepath.Ext(name)
	base := name[:len(name)-len(ext)]
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s.%d%s", base, i, ext)
		destPath = filepath.Join(filesDir, candidate)
		infoPath = filepath.Join(infoDir, candidate+".trashinfo")
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			return
		}
	}
}
