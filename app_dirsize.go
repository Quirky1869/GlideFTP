package main

// Folder size on demand (right-click "Calculate size"). Nothing stores a
// folder's size: every file below it has to be listed and added up. Local =
// filepath.WalkDir; remote = the same kind of parallel breadth-first walk as
// the remote search (one LIST per folder, remoteSearchWorkers connections).
// NAS system folders are skipped like in the search, symlinks are never
// followed. "dirsize:progress" events report the running total; a walk stops
// with "cancelled" when CancelDirSizes is called with its id's prefix.

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"GlideFTP/internal/connection"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// dirSizeMaxDepth stops a remote walk from looping through a symlinked
// folder the listing doesn't flag as a link.
const dirSizeMaxDepth = 64

type DirSizeResult struct {
	Size    int64 `json:"size"`
	Files   int   `json:"files"`
	Partial bool  `json:"partial"` // some folders couldn't be read
}

type dirSizeProgress struct {
	ID    string `json:"id"`
	Size  int64  `json:"size"`
	Files int    `json:"files"`
}

// sizeWalk is the shared state of a remote size walk.
type sizeWalk struct {
	mu       sync.Mutex
	cond     *sync.Cond
	queue    []string
	active   int
	stopped  bool
	size     int64
	files    int
	partial  bool
	failures int // consecutive LIST failures
	lastErr  error
	failed   bool
	lastEmit time.Time
}

func (w *sizeWalk) stop() {
	w.mu.Lock()
	w.stopped = true
	w.cond.Broadcast()
	w.mu.Unlock()
}

func (w *sizeWalk) next() (string, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for !w.stopped && len(w.queue) == 0 && w.active > 0 {
		w.cond.Wait()
	}
	if w.stopped || len(w.queue) == 0 {
		return "", false
	}
	dir := w.queue[0]
	w.queue = w.queue[1:]
	w.active++
	return dir, true
}

// done adds one listed folder; returns true when a progress event is due.
func (w *sizeWalk) done(root string, entries []connection.RemoteFileEntry, err error) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.active--
	defer w.cond.Broadcast()
	if err != nil {
		// An unreadable folder is skipped (partial result); repeated
		// failures mean the server is refusing or dropping us.
		w.partial = true
		w.failures++
		w.lastErr = err
		if w.failures >= remoteSearchMaxErrors*remoteSearchWorkers {
			w.failed = true
			w.stopped = true
		}
		return false
	}
	w.failures = 0
	for _, e := range entries {
		if e.IsDir {
			if !isNASSystemDir(e.Name) && !strings.HasPrefix(e.Mode, "l") &&
				strings.Count(strings.TrimPrefix(e.Path, root), "/") < dirSizeMaxDepth {
				w.queue = append(w.queue, e.Path)
			}
			continue
		}
		w.size += e.Size
		w.files++
	}
	if time.Since(w.lastEmit) > 300*time.Millisecond {
		w.lastEmit = time.Now()
		return true
	}
	return false
}

func (a *App) dirSizeContext(id string) (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	a.dirSizeMu.Lock()
	if a.dirSizeCancels == nil {
		a.dirSizeCancels = map[string]context.CancelFunc{}
	}
	a.dirSizeCancels[id] = cancel
	a.dirSizeMu.Unlock()
	return ctx, func() {
		a.dirSizeMu.Lock()
		delete(a.dirSizeCancels, id)
		a.dirSizeMu.Unlock()
		cancel()
	}
}

// CancelDirSizes stops the size walks whose id starts with prefix (the panel
// passes its side, so leaving a folder doesn't stop the other panel's walk).
func (a *App) CancelDirSizes(prefix string) {
	a.dirSizeMu.Lock()
	defer a.dirSizeMu.Unlock()
	for id, cancel := range a.dirSizeCancels {
		if strings.HasPrefix(id, prefix) {
			cancel()
		}
	}
}

// DirSize adds up the size of every file below path (local or remote).
func (a *App) DirSize(id string, remote bool, path string) (*DirSizeResult, error) {
	ctx, release := a.dirSizeContext(id)
	defer release()
	emit := func(size int64, files int) {
		runtime.EventsEmit(a.ctx, "dirsize:progress", dirSizeProgress{ID: id, Size: size, Files: files})
	}
	if remote {
		return a.remoteDirSize(ctx, path, emit)
	}
	return localDirSize(ctx, path, emit)
}

func localDirSize(ctx context.Context, root string, emit func(int64, int)) (*DirSizeResult, error) {
	res := &DirSizeResult{}
	lastEmit := time.Now()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if p == root {
				return err
			}
			res.Partial = true
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if p != root && isNASSystemDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		// WalkDir never follows symlinks; a link counts as its own small size.
		info, err := d.Info()
		if err != nil {
			res.Partial = true
			return nil
		}
		res.Size += info.Size()
		res.Files++
		if time.Since(lastEmit) > 300*time.Millisecond {
			lastEmit = time.Now()
			emit(res.Size, res.Files)
		}
		return nil
	})
	if ctx.Err() != nil {
		return nil, errors.New("cancelled")
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (a *App) remoteDirSize(ctx context.Context, root string, emit func(int64, int)) (*DirSizeResult, error) {
	// Starting folder on the main connection: a failure here is an error.
	entries, err := a.connMgr.ListDir(root)
	if err != nil {
		return nil, err
	}
	w := &sizeWalk{active: 1} // the starting folder, closed by done() below
	w.cond = sync.NewCond(&w.mu)
	w.done(root, entries, nil)

	var listers []connection.Lister
	if len(w.queue) > 0 {
		if pl, ok := a.connMgr.GetClient().(connection.ParallelLister); ok {
			for i := 0; i < remoteSearchWorkers; i++ {
				l, err := pl.OpenLister()
				if err != nil {
					break
				}
				listers = append(listers, l)
			}
		}
		if len(listers) == 0 {
			listers = append(listers, managerLister{a})
		}
	}
	defer func() {
		for _, l := range listers {
			go l.Close()
		}
	}()

	go func() {
		<-ctx.Done()
		w.stop()
	}()

	var wg sync.WaitGroup
	for _, l := range listers {
		wg.Add(1)
		go func(l connection.Lister) {
			defer wg.Done()
			for {
				dir, ok := w.next()
				if !ok {
					return
				}
				entries, err := l.ListDir(dir)
				if w.done(root, entries, err) {
					w.mu.Lock()
					size, files := w.size, w.files
					w.mu.Unlock()
					emit(size, files)
				}
			}
		}(l)
	}
	wg.Wait()

	if ctx.Err() != nil {
		return nil, errors.New("cancelled")
	}
	if w.failed && w.files == 0 {
		return nil, w.lastErr
	}
	return &DirSizeResult{Size: w.size, Files: w.files, Partial: w.partial}, nil
}
