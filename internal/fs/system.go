package fs

import (
	"os/exec"
	"time"
)

// commandGrace is how long RunCommand waits for the command to fail before
// leaving it running in the background.
const commandGrace = 2 * time.Second

// startAndWatch starts cmd detached from the UI and reports an error only if
// it exits with a failure within grace. The process is always reaped
// (cmd.Wait in a goroutine) so no zombie is left behind.
func startAndWatch(cmd *exec.Cmd, grace time.Duration) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(grace):
		return nil
	}
}
