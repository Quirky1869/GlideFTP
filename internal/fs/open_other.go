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

// OpenURL is not supported on this platform.
func OpenURL(url string) error {
	return errors.New("opening links is not supported on this platform")
}
