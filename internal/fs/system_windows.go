//go:build windows

package fs

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// PowerOff shuts the computer down (no administrator rights needed).
func PowerOff() error {
	cmd := exec.Command("shutdown", "/s", "/t", "0")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return err
	}
	return nil
}

// RunCommand runs a command line typed by the user through cmd.exe, with
// extra environment variables (NAME=value), in the background and without a
// console window. The command line is passed as is (Go's argument quoting
// doesn't follow cmd.exe's rules): /S + outer quotes = cmd.exe removes those
// two quotes only, so the user's own quotes are kept.
func RunCommand(command string, extraEnv []string) error {
	shell := os.Getenv("ComSpec")
	if shell == "" {
		shell = "cmd.exe"
	}
	cmd := exec.Command(shell)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
		CmdLine:    `"` + shell + `" /S /C "` + command + `"`,
	}
	cmd.Env = append(os.Environ(), extraEnv...)
	return startAndWatch(cmd, commandGrace)
}
