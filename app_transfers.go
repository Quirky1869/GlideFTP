package main

import (
	"errors"
	"os"

	"GlideFTP/internal/connection"
	localfs "GlideFTP/internal/fs"
	"GlideFTP/internal/transfer"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Resuming interrupted transfers (point 2) and keeping them across a
// disconnect, a restart or a crash (point 8). Every job is bound to a
// connection (Target, v1.8.0 #135); a paused or failed one is bound again
// to an open connection to the same server account when it is resumed.

// errNotConnected: no open connection to the job's server - the frontend
// asks to connect to it first.
var errNotConnected = errors.New("not_connected")

// targetFor describes connection id as a transfer target.
func (a *App) targetFor(id string) (transfer.Target, bool) {
	_, cfg, ok := a.connMgr.ClientByID(id)
	if !ok {
		return transfer.Target{}, false
	}
	return transfer.Target{ConnID: id, Host: cfg.Host, Account: connection.AccountKey(cfg)}, true
}

// activeTarget is the connection the user is looking at (new transfers).
func (a *App) activeTarget() transfer.Target {
	t, _ := a.targetFor(a.connMgr.ActiveID())
	return t
}

// targetForJob is the job's own connection if still open, else an open
// connection to the same account (the active one first).
func (a *App) targetForJob(job transfer.Job) (transfer.Target, bool) {
	if t, ok := a.targetFor(job.ConnID); ok {
		return t, true
	}
	if job.Account == "" {
		return transfer.Target{}, false
	}
	if t, ok := a.targetFor(a.connMgr.ActiveID()); ok && t.Account == job.Account {
		return t, true
	}
	for _, c := range a.connMgr.GetConnections() {
		if t, ok := a.targetFor(c.ID); ok && t.Account == job.Account {
			return t, true
		}
	}
	return transfer.Target{}, false
}

// RetryTransfer runs a failed or paused transfer again: resume = continue
// where it stopped ("Resume"), else from zero ("Start over").
func (a *App) RetryTransfer(id string, resume bool) error {
	job, ok := a.queue.Get(id)
	if !ok {
		return errors.New("job not found")
	}
	t, ok := a.targetForJob(job)
	if !ok {
		return errNotConnected
	}
	return a.queue.Retry(id, resume, t)
}

// AbandonTransfer drops a failed or paused transfer and deletes the
// incomplete file it left at the destination (it carries the final name, so
// it must not pass for a complete one): local = to the trash, remote =
// deleted if a connection to that server is open. Returns "remote_kept"
// when the remote partial file could not be deleted (not connected).
func (a *App) AbandonTransfer(id string) (string, error) {
	job, ok := a.queue.Get(id)
	if !ok {
		return "", errors.New("job not found")
	}
	if job.Status != transfer.StatusFailed && job.Status != transfer.StatusPaused {
		return "", errors.New("job not abandonable")
	}
	note := ""
	// Only a file this job wrote to: with nothing written, the file there
	// (if any) is not ours - e.g. one "Replace" never got to overwrite.
	if job.BytesDone > 0 {
		if job.Direction == transfer.Download {
			if _, err := os.Stat(job.LocalPath); err == nil {
				if err := localfs.Trash(job.LocalPath); err != nil {
					return "", err
				}
			}
		} else if t, ok := a.targetForJob(job); ok {
			if client, _, ok := a.connMgr.ClientByID(t.ConnID); ok {
				if err := client.Delete(job.RemotePath); err != nil {
					note = "remote_kept"
				}
			}
		} else {
			note = "remote_kept"
		}
	}
	return note, a.queue.RemoveJob(id)
}

// StopConnectionTransfers is the frontend's answer before closing
// connections that still have transfers: pause = "resume later" (kept,
// resumed on the next connection to the server), else stopped (failed).
func (a *App) StopConnectionTransfers(ids []string, pause bool) {
	a.queue.StopConnections(ids, pause)
}

// ResumePausedTransfers restarts, on connection id, the paused transfers of
// its server account (the "Resume N transfers?" banner). Returns how many.
func (a *App) ResumePausedTransfers(id string) int {
	t, ok := a.targetFor(id)
	if !ok {
		return 0
	}
	return a.queue.ResumePaused(t)
}

// QuitWithTransfers is the answer to "app:quitWithTransfers" (see
// beforeClose): stop or pause every transfer, then quit for real.
func (a *App) QuitWithTransfers(pause bool) {
	a.queue.StopAll(pause)
	a.quitConfirmed.Store(true)
	runtime.Quit(a.ctx)
}
