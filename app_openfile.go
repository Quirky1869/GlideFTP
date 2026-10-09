package main

// "Open" a file with the system's default application (like FileZilla's
// View/Edit). Local files are opened in place. Remote files are downloaded to
// os.TempDir()/GlideFTP/open/<id>/<name>, opened, and watched: when the user
// saves them, the frontend is asked whether to send them back (see
// internal/openfile). Design notes: memo/feature-open-with-default-app.md.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"GlideFTP/internal/connection"
	localfs "GlideFTP/internal/fs"
	"GlideFTP/internal/openfile"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Error strings the frontend recognizes (and translates).
const (
	openErrConnectionClosed = "connection_closed"
	openErrRemoteChanged    = "remote_changed"
	openErrCancelled        = "cancelled"
)

// OpenResult tells the frontend what was opened. With NoDefaultApp set, the
// file is ready at Path but the system has no application for it: the
// frontend offers "Open folder" / "Choose an application...".
type OpenResult struct {
	ID           string `json:"id"` // opened remote file ID, "" for a local file
	Path         string `json:"path"`
	NoDefaultApp bool   `json:"noDefaultApp"`
}

// openFileEvent is the payload of the openfile:* events.
type openFileEvent struct {
	openfile.File
	Conflict bool   `json:"conflict"`        // remote file changed since it was opened
	Error    string `json:"error,omitempty"` // openfile:error only
}

func openTempRoot() string {
	return filepath.Join(os.TempDir(), "GlideFTP", "open")
}

// initOpenFiles is called from startup(): removes leftovers of a previous
// session (crash) and starts watching opened files.
func (a *App) initOpenFiles() {
	_ = os.RemoveAll(openTempRoot())
	a.openWatcher = openfile.New(a.onOpenedFileModified)
	a.openWatcher.Start()
}

// cleanupOpenFiles is called from shutdown().
func (a *App) cleanupOpenFiles() {
	if a.openWatcher != nil {
		a.openWatcher.Stop()
	}
	_ = os.RemoveAll(openTempRoot())
}

func toOpenResult(id, p string, err error) (*OpenResult, error) {
	if errors.Is(err, localfs.ErrNoDefaultApp) {
		return &OpenResult{ID: id, Path: p, NoDefaultApp: true}, nil
	}
	if err != nil {
		return nil, err
	}
	return &OpenResult{ID: id, Path: p}, nil
}

// OpenLocalFile opens a local file in place with the default application.
func (a *App) OpenLocalFile(p string) (*OpenResult, error) {
	return toOpenResult("", p, localfs.OpenWithDefault(p))
}

// OpenRemoteFile downloads a remote file of the active connection to a temp
// folder, opens it and starts watching it. Synchronous (the frontend shows a
// loading box); CancelOpenRemoteFile interrupts the download.
func (a *App) OpenRemoteFile(remotePath string) (*OpenResult, error) {
	connID := a.connMgr.ActiveID()
	client, cfg, ok := a.connMgr.ClientByID(connID)
	if !ok {
		return nil, fmt.Errorf("not connected")
	}

	// Already open: reopen the same temp copy, keeping any local edits.
	if f, ok := a.openWatcher.Find(connID, remotePath); ok {
		return toOpenResult(f.ID, f.TempPath, localfs.OpenWithDefault(f.TempPath))
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.openMu.Lock()
	if a.openCancel != nil {
		a.openCancel()
	}
	a.openCancel = cancel
	a.openSeq++
	seq := a.openSeq
	a.openMu.Unlock()
	defer func() {
		a.openMu.Lock()
		if a.openSeq == seq {
			a.openCancel = nil
		}
		a.openMu.Unlock()
		cancel()
	}()

	// Remote state before download, reference for conflict detection.
	remoteMod, remoteSize, _, _ := statRemote(client, remotePath, cfg.TimeoutSec)

	id := uuid.New().String()
	dir := filepath.Join(openTempRoot(), id)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	name := path.Base(remotePath)
	tmp := filepath.Join(dir, safeLocalName(name))
	// Progress for the "Opening x..." box (bar, %, speed, time left):
	// "openfile:progress" events, at most every 200ms plus the final one.
	var lastEmit time.Time
	progress := func(done, total int64) {
		if now := time.Now(); now.Sub(lastEmit) >= 200*time.Millisecond || done >= total {
			lastEmit = now
			runtime.EventsEmit(a.ctx, "openfile:progress", map[string]int64{"done": done, "total": total})
		}
	}
	if err := client.Download(ctx, remotePath, tmp, progress); err != nil {
		_ = os.RemoveAll(dir)
		if ctx.Err() != nil {
			return nil, errors.New(openErrCancelled)
		}
		return nil, err
	}

	a.openWatcher.Add(&openfile.File{
		ID:            id,
		Name:          name,
		TempPath:      tmp,
		RemotePath:    remotePath,
		ConnID:        connID,
		Host:          cfg.Host,
		RemoteModTime: remoteMod,
		RemoteSize:    remoteSize,
	})
	return toOpenResult(id, tmp, localfs.OpenWithDefault(tmp))
}

// CancelOpenRemoteFile interrupts the download of OpenRemoteFile.
func (a *App) CancelOpenRemoteFile() {
	a.openMu.Lock()
	defer a.openMu.Unlock()
	if a.openCancel != nil {
		a.openCancel()
	}
}

// UploadOpenedFile sends an opened file back to the server it came from (its
// original connection, not necessarily the active tab). Unless force is set,
// it first checks the remote file hasn't changed since it was opened and
// returns "remote_changed" if it has.
func (a *App) UploadOpenedFile(id string, force bool) error {
	f, ok := a.openWatcher.Get(id)
	if !ok {
		return fmt.Errorf("file is no longer open")
	}
	client, cfg, ok := a.connMgr.ClientByID(f.ConnID)
	if !ok {
		a.openWatcher.Ignore(id)
		return errors.New(openErrConnectionClosed)
	}

	if !force && !f.RemoteModTime.IsZero() {
		mod, size, found, err := statRemote(client, f.RemotePath, cfg.TimeoutSec)
		if err == nil && found && (!mod.Equal(f.RemoteModTime) || size != f.RemoteSize) {
			return errors.New(openErrRemoteChanged)
		}
	}

	if err := client.Upload(context.Background(), f.TempPath, f.RemotePath, nil); err != nil {
		a.openWatcher.Ignore(id)
		return err
	}
	mod, size, _, _ := statRemote(client, f.RemotePath, cfg.TimeoutSec)
	a.openWatcher.MarkUploaded(id, mod, size)
	return nil
}

// IgnoreOpenedFileChange: the user answered "No" (or cancelled a conflict).
func (a *App) IgnoreOpenedFileChange(id string) {
	a.openWatcher.Ignore(id)
}

// SetOpenedFileAutoUpload: "Always for this file" - later saves are sent
// without asking.
func (a *App) SetOpenedFileAutoUpload(id string, on bool) {
	a.openWatcher.SetAutoUpload(id, on)
}

// ChooseAppAndOpen lets the user pick an executable and opens p with it -
// used when the system has no application for the file type.
func (a *App) ChooseAppAndOpen(p string) error {
	title := "Choose an application"
	if a.appSettings.Language == "fr" {
		title = "Choisir une application"
	}
	opts := runtime.OpenDialogOptions{Title: title}
	if goruntime.GOOS == "linux" {
		opts.DefaultDirectory = "/usr/bin"
	}
	app, err := runtime.OpenFileDialog(a.ctx, opts)
	if err != nil || app == "" {
		return err
	}
	return localfs.OpenWithApp(app, p)
}

// OpenContainingFolder opens the folder containing p in the file manager
// (which always offers "Open with...").
func (a *App) OpenContainingFolder(p string) error {
	return localfs.OpenWithDefault(filepath.Dir(p))
}

// onOpenedFileModified is the watcher callback (polling goroutine).
func (a *App) onOpenedFileModified(f openfile.File) {
	if !f.AutoUpload {
		runtime.EventsEmit(a.ctx, "openfile:modified", openFileEvent{File: f})
		return
	}
	err := a.UploadOpenedFile(f.ID, false)
	switch {
	case err == nil:
		runtime.EventsEmit(a.ctx, "openfile:uploaded", openFileEvent{File: f})
	case err.Error() == openErrRemoteChanged:
		// Never overwrite a server-side change silently, even in auto mode.
		runtime.EventsEmit(a.ctx, "openfile:modified", openFileEvent{File: f, Conflict: true})
	default:
		runtime.EventsEmit(a.ctx, "openfile:error", openFileEvent{File: f, Error: err.Error()})
	}
}

// forgetOpenedFiles stops watching the files opened from a connection that
// is being closed or replaced, and warns the frontend if some had changes
// that were never sent back.
func (a *App) forgetOpenedFiles(connID string) {
	if connID == "" || a.openWatcher == nil {
		return
	}
	unsynced := a.openWatcher.RemoveByConn(connID)
	if len(unsynced) == 0 {
		return
	}
	names := make([]string, len(unsynced))
	for i, f := range unsynced {
		names[i] = f.Name
	}
	runtime.EventsEmit(a.ctx, "openfile:orphaned", map[string]interface{}{
		"host":  unsynced[0].Host,
		"names": names,
	})
}

// beforeClose (options.App.OnBeforeClose): warns when opened files have
// changes that were never sent back. Returns true to cancel quitting.
func (a *App) beforeClose(ctx context.Context) bool {
	if a.openWatcher == nil {
		return false
	}
	n := len(a.openWatcher.Unsynced())
	if n == 0 {
		return false
	}
	title := "Unsent changes"
	msg := fmt.Sprintf("%d modified file(s) opened from the server have not been sent back. Their temporary copies will be deleted. Quit anyway?", n)
	if a.appSettings.Language == "fr" {
		title = "Modifications non renvoyées"
		msg = fmt.Sprintf("%d fichier(s) modifié(s) ouvert(s) depuis le serveur n'ont pas été renvoyés. Leurs copies temporaires seront supprimées. Quitter quand même ?", n)
	}
	choice, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:    runtime.QuestionDialog,
		Title:   title,
		Message: msg,
	})
	if err != nil {
		return false
	}
	return choice != "Yes"
}

// statRemote returns a remote file's ModTime/Size by listing its parent
// directory (works for FTP and SFTP alike), bounded by the operation timeout.
func statRemote(client connection.Client, remotePath string, timeoutSec int) (time.Time, int64, bool, error) {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	type res struct {
		entries []connection.RemoteFileEntry
		err     error
	}
	ch := make(chan res, 1)
	go func() {
		entries, err := client.ListDir(path.Dir(remotePath))
		ch <- res{entries, err}
	}()
	var r res
	select {
	case r = <-ch:
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		return time.Time{}, 0, false, fmt.Errorf("operation timed out")
	}
	if r.err != nil {
		return time.Time{}, 0, false, r.err
	}
	name := path.Base(remotePath)
	for _, e := range r.entries {
		if e.Name == name {
			return e.ModTime, e.Size, true, nil
		}
	}
	return time.Time{}, 0, false, nil
}

// safeLocalName makes a remote file name usable as a local file name:
// characters Windows forbids are replaced (a remote "a:b.txt" is legal on
// a Linux server).
func safeLocalName(name string) string {
	if goruntime.GOOS != "windows" {
		return name
	}
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			return '_'
		}
		return r
	}, name)
}
