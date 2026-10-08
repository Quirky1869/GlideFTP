// Package openfile tracks remote files downloaded to a temp folder and opened
// with the system's default application, and detects when they are modified
// so the changes can be sent back to the server.
//
// Detection polls each temp file's ModTime + Size rather than using
// inotify/fsnotify: many editors save atomically (write a new file then
// rename it over the old one), which breaks watches set on the file itself.
// We also never try to detect the editor closing: openers (xdg-open,
// ShellExecute) return immediately and single-instance editors hand the file
// to an already running process, so "process exited" means nothing.
package openfile

import (
	"os"
	"sync"
	"time"
)

// PollInterval is the delay between two checks of the opened files.
const PollInterval = 2 * time.Second

// File is one opened remote file. The exported fields are sent to the frontend.
type File struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	TempPath   string `json:"tempPath"`
	RemotePath string `json:"remotePath"`
	ConnID     string `json:"connId"`
	Host       string `json:"host"`
	AutoUpload bool   `json:"autoUpload"`

	// Remote state when downloaded / last uploaded, for conflict detection.
	RemoteModTime time.Time `json:"-"`
	RemoteSize    int64     `json:"-"`

	// Local state of the temp file at download / last upload / last "No".
	lastModTime time.Time
	lastSize    int64
	// Debounce: state seen at the previous poll while it differed from last.
	seenModTime time.Time
	seenSize    int64
	seen        bool
	// A change was reported and is awaiting the user's decision / an upload.
	pending bool
	// Local changes exist that never reached the server (user said "No", or
	// the upload failed). Counts for the "unsent changes" warnings only.
	dirty bool
}

type Watcher struct {
	mu         sync.Mutex
	files      map[string]*File
	onModified func(File)
	stop       chan struct{}
}

// New creates a watcher; onModified is called (outside the lock, from the
// polling goroutine) once per detected, stable modification.
func New(onModified func(File)) *Watcher {
	return &Watcher{files: map[string]*File{}, onModified: onModified}
}

func (w *Watcher) Start() {
	w.mu.Lock()
	if w.stop != nil {
		w.mu.Unlock()
		return
	}
	w.stop = make(chan struct{})
	stop := w.stop
	w.mu.Unlock()

	go func() {
		ticker := time.NewTicker(PollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				w.poll()
			}
		}
	}()
}

func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stop != nil {
		close(w.stop)
		w.stop = nil
	}
}

// poll reports a file once its new state has been identical for two
// consecutive polls - editors sometimes write a file in several steps.
func (w *Watcher) poll() {
	var changed []File
	w.mu.Lock()
	for _, f := range w.files {
		info, err := os.Stat(f.TempPath)
		if err != nil {
			continue // briefly missing during an atomic save, or deleted
		}
		mod, size := info.ModTime(), info.Size()
		if mod.Equal(f.lastModTime) && size == f.lastSize {
			f.seen = false
			continue
		}
		if f.pending {
			continue // already reported; the upload will take the latest content
		}
		if !f.seen || !mod.Equal(f.seenModTime) || size != f.seenSize {
			f.seen, f.seenModTime, f.seenSize = true, mod, size
			continue
		}
		f.pending = true
		f.seen = false
		changed = append(changed, *f)
	}
	w.mu.Unlock()

	for _, f := range changed {
		w.onModified(f)
	}
}

// Add starts watching f. Its current local state is the "unmodified" baseline.
func (w *Watcher) Add(f *File) {
	if info, err := os.Stat(f.TempPath); err == nil {
		f.lastModTime, f.lastSize = info.ModTime(), info.Size()
	}
	w.mu.Lock()
	w.files[f.ID] = f
	w.mu.Unlock()
}

// Get returns a copy of the file with this ID.
func (w *Watcher) Get(id string) (File, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	f, ok := w.files[id]
	if !ok {
		return File{}, false
	}
	return *f, true
}

// Find returns the opened file for this connection + remote path, if any.
func (w *Watcher) Find(connID, remotePath string) (File, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, f := range w.files {
		if f.ConnID == connID && f.RemotePath == remotePath {
			return *f, true
		}
	}
	return File{}, false
}

// resetBaseline makes the temp file's current state the new "unmodified"
// state and clears the pending flag. Caller must hold w.mu.
func (f *File) resetBaseline() {
	if info, err := os.Stat(f.TempPath); err == nil {
		f.lastModTime, f.lastSize = info.ModTime(), info.Size()
	}
	f.pending = false
	f.seen = false
}

// MarkUploaded records a successful upload: the local file is in sync and
// the new remote state is the conflict-detection reference.
func (w *Watcher) MarkUploaded(id string, remoteMod time.Time, remoteSize int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if f, ok := w.files[id]; ok {
		f.resetBaseline()
		f.dirty = false
		f.RemoteModTime, f.RemoteSize = remoteMod, remoteSize
	}
}

// Ignore drops the current modification ("No", or a failed upload): nothing
// is asked again until the file is saved once more. The file stays marked as
// having unsent changes for the warnings on tab close / app quit.
func (w *Watcher) Ignore(id string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if f, ok := w.files[id]; ok {
		f.resetBaseline()
		f.dirty = true
	}
}

func (w *Watcher) SetAutoUpload(id string, on bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if f, ok := w.files[id]; ok {
		f.AutoUpload = on
	}
}

// isUnsynced: modified since the last download/upload/ignore. Caller holds w.mu.
func (f *File) isUnsynced() bool {
	if f.pending || f.dirty {
		return true
	}
	info, err := os.Stat(f.TempPath)
	if err != nil {
		return false
	}
	return !info.ModTime().Equal(f.lastModTime) || info.Size() != f.lastSize
}

// Unsynced returns the files with modifications not sent to the server.
func (w *Watcher) Unsynced() []File {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []File
	for _, f := range w.files {
		if f.isUnsynced() {
			out = append(out, *f)
		}
	}
	return out
}

// RemoveAll stops watching every file.
func (w *Watcher) RemoveAll() {
	w.mu.Lock()
	w.files = map[string]*File{}
	w.mu.Unlock()
}

// RemoveByConn stops watching every file opened from this connection and
// returns those that had unsent modifications.
func (w *Watcher) RemoveByConn(connID string) (unsynced []File) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for id, f := range w.files {
		if f.ConnID != connID {
			continue
		}
		if f.isUnsynced() {
			unsynced = append(unsynced, *f)
		}
		delete(w.files, id)
	}
	return unsynced
}
