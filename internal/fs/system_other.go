//go:build !linux && !windows

package fs

import "errors"

// PowerOff is not supported on this platform.
func PowerOff() error {
	return errors.New("shutting down is not supported on this platform")
}

// RunCommand is not supported on this platform.
func RunCommand(command string, extraEnv []string) error {
	return errors.New("running commands is not supported on this platform")
}
