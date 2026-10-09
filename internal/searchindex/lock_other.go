//go:build !windows

package searchindex

import "os"

// openLocked opens (creating it if needed) path for reading and writing.
// Linux has no way to stop the file's owner from editing, renaming or
// deleting it (file locks are advisory only; mandatory locking was removed
// from the kernel in 5.15): the integrity MAC is what makes such edits
// harmless there - see Index.load.
func openLocked(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
}
