//go:build linux

package fs

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// PowerOff shuts the computer down. systemctl asks logind, which lets the
// user of the active local session do it without root (polkit); loginctl is
// the same request for systems where systemctl is missing.
func PowerOff() error {
	var errs []string
	for _, args := range [][]string{{"systemctl", "poweroff"}, {"loginctl", "poweroff"}} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Env = cleanEnv(os.Environ())
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		errs = append(errs, args[0]+": "+msg)
	}
	return fmt.Errorf("%s", strings.Join(errs, "; "))
}

// RunCommand runs a command line typed by the user through sh, with extra
// environment variables (NAME=value), in the background.
func RunCommand(command string, extraEnv []string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = append(cleanEnv(os.Environ()), extraEnv...)
	return startAndWatch(cmd, commandGrace)
}
