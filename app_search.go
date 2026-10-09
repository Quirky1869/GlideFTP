package main

// Remote search. FTP has no server-side search command: the only way is to
// list every folder, one LIST round-trip (plus a new data connection) each -
// over the internet that's easily 0.2-0.5s per folder. So the walk:
//   - runs remoteSearchWorkers listings in parallel, each worker on its own
//     connection (connection.ParallelLister; FTP can't pipeline commands);
//   - is breadth-first, so folders near the starting one are searched first;
//   - streams matches to the panel as they're found ("search:progress"
//     event) instead of only returning at the very end;
//   - skips NAS system folders (snapshots = full copies of the share);
//   - runs one search at a time, cancelled by the next one or CancelSearch;
//   - with the search index enabled (Settings > Recherche), first lists the
//     folders where previous listings saw a matching name. Listing them is
//     the verification: only what the server returns now is shown. Every
//     listing refreshes the index; the full walk still follows, so files
//     added or moved since are found too.

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/textproto"
	"os"
	"strings"
	"sync"

	"GlideFTP/internal/connection"
	"GlideFTP/internal/searchindex"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	remoteSearchWorkers     = 4    // parallel listings (extra FTP connections)
	remoteSearchResultLimit = 500  // matches
	remoteSearchDirLimit    = 3000 // folders listed
	remoteSearchMaxErrors   = 3    // consecutive LIST failures (per worker) = connection in trouble
)

// RemoteSearchResult is returned by RemoteSearch. Truncated: a limit was hit
// (or the walk had to stop on errors) before the whole tree was searched.
type RemoteSearchResult struct {
	Entries   []connection.RemoteFileEntry `json:"entries"`
	Truncated bool                         `json:"truncated"`
}

// searchProgress is the "search:progress" event payload.
type searchProgress struct {
	ID      string                       `json:"id"`
	Entries []connection.RemoteFileEntry `json:"entries"` // new matches since the previous event
	Listed  int                          `json:"listed"`  // folders listed so far
}

// isNASSystemDir: NAS housekeeping folders a recursive search must not walk
// into. Snapshot folders (QNAP @Recently-Snapshot, Synology #snapshot) hold
// a full copy of the share per snapshot - huge walks and duplicate matches -
// and recycle bins / thumbnail caches aren't what users search for.
func isNASSystemDir(name string) bool {
	n := strings.ToLower(name)
	return strings.HasPrefix(n, "@") || // QNAP @Recently-Snapshot, @Recycle; Synology @eaDir, @tmp...
		strings.HasPrefix(n, ".@") || // QNAP .@__thumb, .@__qini
		n == "#recycle" || n == "#snapshot" // Synology
}

// indexForActive returns the search index of the active connection's server,
// or nil when the option is off.
func (a *App) indexForActive() *searchindex.Index {
	if a.searchIdx == nil || !a.appSettings.SearchIndexEnabled {
		return nil
	}
	_, cfg, ok := a.connMgr.ClientByID(a.connMgr.ActiveID())
	if !ok {
		return nil
	}
	return a.searchIdx.Get(searchindex.Key(string(cfg.Protocol), cfg.Host, cfg.Port, cfg.User))
}

func toIndexEntries(entries []connection.RemoteFileEntry) []searchindex.Entry {
	out := make([]searchindex.Entry, len(entries))
	for i, e := range entries {
		out[i] = searchindex.Entry{Name: e.Name, IsDir: e.IsDir, Size: e.Size, ModTime: e.ModTime}
	}
	return out
}

// isNotFound: the folder no longer exists (FTP 550, SFTP "no such file").
// 550 also covers "permission denied" - forgetting such a folder in the
// index is harmless, it can't be searched anyway.
func isNotFound(err error) bool {
	var tpErr *textproto.Error
	if errors.As(err, &tpErr) {
		return tpErr.Code == 550
	}
	return errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err)
}

// hasNASSystemSegment: a path going through a NAS system folder.
func hasNASSystemSegment(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg != "" && isNASSystemDir(seg) {
			return true
		}
	}
	return false
}

// SearchIndexInfo is shown in Settings > Recherche and in the explanation popup.
type SearchIndexInfo struct {
	Dir       string `json:"dir"`
	SizeBytes int64  `json:"sizeBytes"`
}

func (a *App) GetSearchIndexInfo() SearchIndexInfo {
	info := SearchIndexInfo{Dir: searchindex.Dir()}
	if a.searchIdx != nil {
		a.searchIdx.Flush() // so the size reflects what is in memory
		info.SizeBytes = a.searchIdx.SizeBytes()
	}
	return info
}

// ClearSearchIndex deletes every remembered folder, on disk and in memory.
func (a *App) ClearSearchIndex() error {
	if a.searchIdx == nil {
		return nil
	}
	return a.searchIdx.Clear()
}

// searchWalk is the shared state of the parallel breadth-first walk.
type searchWalk struct {
	mu        sync.Mutex
	cond      *sync.Cond
	queue     []string
	seen      map[string]bool // folders already handed out (index candidates are also met again by the walk)
	active    int // folders being listed right now
	stopped   bool
	listed    int
	failures  int
	lastErr   error
	results   []connection.RemoteFileEntry
	truncated bool
	failedOut bool // stopped because of repeated listing failures
}

func (w *searchWalk) stopLocked() {
	w.stopped = true
	w.cond.Broadcast()
}

func (w *searchWalk) stop() {
	w.mu.Lock()
	w.stopLocked()
	w.mu.Unlock()
}

// next hands out the next folder to list. It waits while the queue is empty
// but other workers may still add subfolders; false = nothing left / stopped.
// Folders already listed in this search (index candidates met again by the
// walk) are skipped.
func (w *searchWalk) next() (string, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for {
		for !w.stopped && len(w.queue) == 0 && w.active > 0 {
			w.cond.Wait()
		}
		if w.stopped || len(w.queue) == 0 {
			return "", false
		}
		dir := w.queue[0]
		w.queue = w.queue[1:]
		if w.seen[dir] {
			continue
		}
		if w.listed >= remoteSearchDirLimit {
			w.truncated = true
			w.stopLocked()
			return "", false
		}
		w.seen[dir] = true
		w.active++
		w.listed++
		return dir, true
	}
}

// done records one listed folder: queues its subfolders, keeps the matches
// (cut at the result limit) and returns those to report + the listed count.
func (w *searchWalk) done(subdirs []string, matches []connection.RemoteFileEntry, err error) ([]connection.RemoteFileEntry, int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.active--
	defer w.cond.Broadcast()

	if err != nil {
		// One unreadable folder (permissions) is skipped; repeated failures
		// mean the server is refusing or dropping us - stop.
		w.failures++
		w.lastErr = err
		if w.failures >= remoteSearchMaxErrors*remoteSearchWorkers {
			w.truncated = true
			w.failedOut = true
			w.stopLocked()
		}
		return nil, w.listed
	}
	w.failures = 0
	if !w.stopped {
		w.queue = append(w.queue, subdirs...)
	}
	if room := remoteSearchResultLimit - len(w.results); len(matches) > room {
		matches = matches[:room]
	}
	w.results = append(w.results, matches...)
	if len(w.results) >= remoteSearchResultLimit {
		w.truncated = true
		w.stopLocked()
	}
	return matches, w.listed
}

// managerLister lists through the connection manager (main connection) -
// fallback when no dedicated lister can be opened.
type managerLister struct{ a *App }

func (l managerLister) ListDir(p string) ([]connection.RemoteFileEntry, error) {
	return l.a.connMgr.ListDir(p)
}
func (l managerLister) Close() error { return nil }

func splitEntries(entries []connection.RemoteFileEntry, q string, recursive bool) (subdirs []string, matches []connection.RemoteFileEntry) {
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Name), q) {
			matches = append(matches, e)
		}
		if recursive && e.IsDir && !isNASSystemDir(e.Name) {
			subdirs = append(subdirs, e.Path)
		}
	}
	return subdirs, matches
}

// RemoteSearch looks for entries whose name contains query (case-insensitive)
// under path - its direct children, or the whole tree when recursive.
// searchID tags the "search:progress" events so the panel only takes its
// own. A new call cancels the previous search, which returns "cancelled".
func (a *App) RemoteSearch(searchID, path, query string, recursive bool) (*RemoteSearchResult, error) {
	ctx, cancel := context.WithCancel(context.Background())
	a.searchMu.Lock()
	if a.searchCancel != nil {
		a.searchCancel()
	}
	a.searchCancel = cancel
	a.searchSeq++
	seq := a.searchSeq
	a.searchMu.Unlock()
	defer func() {
		a.searchMu.Lock()
		if a.searchSeq == seq {
			a.searchCancel = nil
		}
		a.searchMu.Unlock()
		cancel()
	}()

	q := strings.ToLower(query)
	connID := a.connMgr.ActiveID()
	idx := a.indexForActive() // nil when the option is off
	if idx != nil {
		defer func() { go a.searchIdx.Flush() }()
	}
	emit := func(matches []connection.RemoteFileEntry, listed int) {
		runtime.EventsEmit(a.ctx, "search:progress", searchProgress{ID: searchID, Entries: matches, Listed: listed})
	}

	// Starting folder on the main connection: a failure here is an error.
	entries, err := a.connMgr.ListDir(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	subdirs, matches := splitEntries(entries, q, recursive)
	if idx != nil {
		idx.Put(path, toIndexEntries(entries))
	}

	// Folders where the index saw a matching name are listed first: if the
	// file is still there, it shows up within a couple of round-trips.
	queue := subdirs
	if idx != nil && recursive {
		cands := idx.CandidateDirs(q, path, func(dir string) bool {
			return dir == path || hasNASSystemSegment(strings.TrimPrefix(dir, path))
		})
		queue = append(cands, subdirs...)
	}

	w := &searchWalk{listed: 1, queue: queue, seen: map[string]bool{path: true}}
	w.cond = sync.NewCond(&w.mu)
	if len(matches) > remoteSearchResultLimit {
		matches = matches[:remoteSearchResultLimit]
		w.truncated = true
	}
	w.results = matches
	emit(matches, 1)
	if !recursive || w.truncated {
		return &RemoteSearchResult{Entries: w.results, Truncated: w.truncated}, nil
	}

	// Workers: dedicated listers when the client supports them.
	var listers []connection.Lister
	if pl, ok := a.connMgr.GetClient().(connection.ParallelLister); ok {
		for i := 0; i < remoteSearchWorkers; i++ {
			l, err := pl.OpenLister()
			if err != nil {
				break // server limit reached: go on with what we have
			}
			listers = append(listers, l)
		}
	}
	if len(listers) == 0 {
		listers = append(listers, managerLister{a})
	}
	defer func() {
		for _, l := range listers {
			go l.Close()
		}
	}()

	// Cancellation (new search, search closed, tab switch) stops the walk;
	// listings already in flight finish, nothing new starts.
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
				if a.connMgr.ActiveID() != connID {
					cancel()
				}
				entries, err := l.ListDir(dir)
				var subdirs []string
				var matches []connection.RemoteFileEntry
				if err != nil {
					if idx != nil && isNotFound(err) {
						idx.RemoveTree(dir) // gone from the server (or no longer readable)
					}
					err = fmt.Errorf("%s: %w", dir, err)
				} else {
					subdirs, matches = splitEntries(entries, q, true)
					if idx != nil {
						idx.Put(dir, toIndexEntries(entries))
					}
				}
				reported, listed := w.done(subdirs, matches, err)
				if len(reported) > 0 || listed%10 == 0 {
					emit(reported, listed)
				}
			}
		}(l)
	}
	wg.Wait()

	if ctx.Err() != nil {
		return nil, errors.New("cancelled")
	}
	if w.failedOut && len(w.results) == 0 {
		return nil, w.lastErr
	}
	return &RemoteSearchResult{Entries: w.results, Truncated: w.truncated}, nil
}

// CancelSearch stops the remote search in progress (search closed by the user).
func (a *App) CancelSearch() {
	a.searchMu.Lock()
	defer a.searchMu.Unlock()
	if a.searchCancel != nil {
		a.searchCancel()
	}
}
