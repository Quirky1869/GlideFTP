//go:build !linux && !windows

package fs

// Trash falls back to permanent deletion on unsupported platforms.
func Trash(path string) error {
	return Delete(path)
}
