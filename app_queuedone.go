package main

import (
	"fmt"

	localfs "GlideFTP/internal/fs"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Actions after the transfer queue is finished, chosen by right-clicking the
// queue (stores/queueDone.js). The countdowns before quitting or shutting
// down are run by the frontend; nothing is sent to the system before they end.

// UnsentOpenedFileCount: files opened from the server whose changes were
// never sent back (shown in the shutdown countdown, their temporary copies
// are lost).
func (a *App) UnsentOpenedFileCount() int {
	if a.openWatcher == nil {
		return 0
	}
	return len(a.openWatcher.Unsynced())
}

// ShutdownComputer saves the kept transfers (queue.json) then turns the
// computer off. On success GlideFTP quits on its own, the system closing it
// anyway; an error (no permission, shutdown inhibited...) is returned as is.
func (a *App) ShutdownComputer() error {
	if a.queue != nil {
		a.queue.Save()
	}
	if err := localfs.PowerOff(); err != nil {
		return err
	}
	a.quitConfirmed.Store(true)
	runtime.Quit(a.ctx)
	return nil
}

// RunQueueDoneCommand runs the user's command line in the background, with
// the outcome of the queue in GLIDEFTP_DONE / GLIDEFTP_FAILED. Those are only
// given to that process, they are not stored anywhere.
func (a *App) RunQueueDoneCommand(command string, done, failed int) error {
	return localfs.RunCommand(command, []string{
		fmt.Sprintf("GLIDEFTP_DONE=%d", done),
		fmt.Sprintf("GLIDEFTP_FAILED=%d", failed),
	})
}
