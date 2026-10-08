//go:build !linux && !windows

package fs

import "errors"

// OpenWithDefault is not supported on this platform.
func OpenWithDefault(path string) error {
	return errors.New("opening files is not supported on this platform")
}

// OpenWithApp is not supported on this platform.
func OpenWithApp(appPath, filePath string) error {
	return errors.New("opening files is not supported on this platform")
}
