// Package searchindex remembers, per server, the content of the remote
// folders GlideFTP has listed (during searches and normal navigation), so a
// later search can check the already known locations first instead of
// walking the whole tree before showing anything.
//
// The index is only a hint: the search lists the folders it points to on the
// server and shows what is really there, never what the index claims. Every
// listing then refreshes the index (removed/moved files disappear, new ones
// appear).
//
// Storage (opt-in, Settings > Recherche): one gzipped JSON file per server in
// the OS cache directory -
//   Linux:   ~/.cache/GlideFTP/search-index/<key>.json.gz
//   Windows: %LOCALAPPDATA%\GlideFTP\search-index\<key>.json.gz
// It contains file and folder names, sizes and dates in clear - never
// passwords or file contents.
//
// Protection of these files:
//   - Integrity: each file carries an HMAC-SHA256 of its content, keyed with
//     a random per-device key (index.key, same folder). A file modified
//     outside GlideFTP (or half-written by a crash) fails the check and is
//     discarded, then rebuilt - never used. And since the index never produces
//     a result on its own (everything is verified on the server), even an
//     undetected edit could only make a search list a few useless folders.
//   - Windows: while GlideFTP runs, every index file and the key are kept
//     open with share mode 0 (lock_windows.go): other programs can't read,
//     modify, delete or rename them. Linux can't enforce that (lock_other.go).
//   - Files are created 0600 (owner only).
package searchindex

import (
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FlushInterval is how often modified indexes are written to disk.
const FlushInterval = 30 * time.Second

// Entry is one remembered file or folder (short JSON keys keep files small).
type Entry struct {
	Name    string    `json:"n"`
	IsDir   bool      `json:"d,omitempty"`
	Size    int64     `json:"s,omitempty"`
	ModTime time.Time `json:"t"`
}

type dirRecord struct {
	Listed  time.Time `json:"l"`
	Entries []Entry   `json:"e"`
}

// fileFormat is the (gzipped) content of an index file. MAC = HMAC-SHA256
// of the exact Dirs bytes with the device key.
type fileFormat struct {
	V    int             `json:"v"`
	MAC  string          `json:"mac"`
	Dirs json.RawMessage `json:"dirs"`
}

const (
	formatVersion = 1
	keyFileName   = "index.key"
	indexExt      = ".json.gz"
)

// Index is the remembered tree of one server.
type Index struct {
	mu    sync.Mutex
	f     *os.File // kept open for the whole session (Windows: locked)
	key   []byte
	dirs  map[string]*dirRecord
	dirty bool
}

// Dir is where the index files are stored.
func Dir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "GlideFTP", "search-index")
}

// Key identifies a server account: same host with another user or protocol
// = another index.
func Key(protocol, host string, port int, user string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%s", protocol, strings.ToLower(host), port, user)))
	return hex.EncodeToString(sum[:12])
}

// under reports whether dir is root or inside it ("/" contains everything).
func under(dir, root string) bool {
	root = strings.TrimSuffix(root, "/")
	if root == "" {
		return true
	}
	return dir == root || strings.HasPrefix(dir, root+"/")
}

// Put replaces what is known about dir with a fresh listing.
func (x *Index) Put(dir string, entries []Entry) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.dirs[dir] = &dirRecord{Listed: time.Now(), Entries: entries}
	x.dirty = true
}

// RemoveTree forgets dir and everything below it (folder gone from the server).
func (x *Index) RemoveTree(dir string) {
	x.mu.Lock()
	defer x.mu.Unlock()
	for d := range x.dirs {
		if under(d, dir) {
			delete(x.dirs, d)
			x.dirty = true
		}
	}
}

// CandidateDirs returns the folders under root that, according to the index,
// contain an entry whose name contains q (lowercase) - the folders a search
// should list first. skip filters out folders the search must not enter.
func (x *Index) CandidateDirs(q, root string, skip func(dir string) bool) []string {
	x.mu.Lock()
	defer x.mu.Unlock()
	var out []string
	for dir, rec := range x.dirs {
		if !under(dir, root) || (skip != nil && skip(dir)) {
			continue
		}
		for _, e := range rec.Entries {
			if strings.Contains(strings.ToLower(e.Name), q) {
				out = append(out, dir)
				break
			}
		}
	}
	return out
}

func mac(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// load reads the index through its open handle. Anything unreadable,
// from another format, or failing the MAC check (edited outside GlideFTP,
// written with another device key, half-written) is discarded: the index
// starts empty and is rebuilt by the next listings.
func (x *Index) load() {
	if _, err := x.f.Seek(0, io.SeekStart); err != nil {
		return
	}
	zr, err := gzip.NewReader(x.f)
	if err != nil {
		return
	}
	defer zr.Close()
	var ff fileFormat
	if json.NewDecoder(zr).Decode(&ff) != nil || ff.V != formatVersion {
		return
	}
	want, err := hex.DecodeString(ff.MAC)
	if err != nil || !hmac.Equal(want, mac(x.key, ff.Dirs)) {
		return
	}
	var dirs map[string]*dirRecord
	if json.Unmarshal(ff.Dirs, &dirs) == nil && dirs != nil {
		x.dirs = dirs
	}
}

// flush rewrites the file in place through the open handle (a temp file +
// rename isn't possible while the file is locked on Windows). A crash in
// the middle leaves a file that fails the MAC check: discarded, rebuilt.
func (x *Index) flush() error {
	x.mu.Lock()
	if !x.dirty || x.f == nil {
		x.mu.Unlock()
		return nil
	}
	dirs, err := json.Marshal(x.dirs)
	x.dirty = false
	key := x.key
	f := x.f
	x.mu.Unlock()
	if err != nil {
		return err
	}
	content, err := json.Marshal(fileFormat{V: formatVersion, MAC: hex.EncodeToString(mac(key, dirs)), Dirs: dirs})
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write(content)
	if err := zw.Close(); err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		return err
	}
	return f.Sync()
}

func (x *Index) close() {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.f != nil {
		x.f.Close()
		x.f = nil
	}
}

// Manager keeps the indexes loaded in memory (and their files open) and
// flushes them periodically.
type Manager struct {
	mu      sync.Mutex
	open    map[string]*Index
	key     []byte
	keyFile *os.File
	stop    chan struct{}
}

// NewManager starts the periodic flush. If indexes already exist on this
// device, they are all opened right away - even before any connection - so
// they are protected (locked on Windows) for the whole session. Nothing is
// created on disk until an index is actually used (option enabled).
func NewManager() *Manager {
	m := &Manager{open: map[string]*Index{}, stop: make(chan struct{})}
	if entries, err := os.ReadDir(Dir()); err == nil {
		for _, e := range entries {
			if name := e.Name(); !e.IsDir() && strings.HasSuffix(name, indexExt) {
				m.Get(strings.TrimSuffix(name, indexExt))
			}
		}
	}
	go func() {
		t := time.NewTicker(FlushInterval)
		defer t.Stop()
		for {
			select {
			case <-m.stop:
				return
			case <-t.C:
				m.Flush()
			}
		}
	}()
	return m
}

// ensureKeyLocked opens (or creates) the per-device MAC key. Caller holds m.mu.
func (m *Manager) ensureKeyLocked() error {
	if m.key != nil {
		return nil
	}
	if err := os.MkdirAll(Dir(), 0700); err != nil {
		return err
	}
	f, err := openLocked(filepath.Join(Dir(), keyFileName))
	if err != nil {
		return err
	}
	key, _ := io.ReadAll(f)
	if len(key) != 32 {
		// New device (or damaged key): new key - existing index files then
		// fail their MAC check and are rebuilt.
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			f.Close()
			return err
		}
		f.Truncate(0)
		f.Seek(0, io.SeekStart)
		if _, err := f.Write(key); err != nil {
			f.Close()
			return err
		}
		f.Sync()
	}
	m.key, m.keyFile = key, f
	return nil
}

// Get returns the index for a server key, opening and loading its file on
// first use. If the file can't be opened, the index still works in memory.
func (m *Manager) Get(key string) *Index {
	m.mu.Lock()
	defer m.mu.Unlock()
	if x, ok := m.open[key]; ok {
		return x
	}
	x := &Index{dirs: map[string]*dirRecord{}}
	if m.ensureKeyLocked() == nil {
		x.key = m.key
		if f, err := openLocked(filepath.Join(Dir(), key+indexExt)); err == nil {
			x.f = f
			x.load()
		}
	}
	m.open[key] = x
	return x
}

// Flush writes every modified index to disk.
func (m *Manager) Flush() {
	m.mu.Lock()
	list := make([]*Index, 0, len(m.open))
	for _, x := range m.open {
		list = append(list, x)
	}
	m.mu.Unlock()
	for _, x := range list {
		_ = x.flush()
	}
}

func (m *Manager) closeAllLocked() {
	for _, x := range m.open {
		x.close()
	}
	if m.keyFile != nil {
		m.keyFile.Close()
		m.keyFile = nil
	}
	m.key = nil
}

// Close flushes, releases the files and stops the periodic flush (shutdown).
func (m *Manager) Close() {
	close(m.stop)
	m.Flush()
	m.mu.Lock()
	m.closeAllLocked()
	m.mu.Unlock()
}

// Clear forgets everything: in memory and on disk (files are closed first -
// on Windows they can't be deleted while open). The next use creates a new
// key and new files.
func (m *Manager) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.open {
		x.mu.Lock()
		x.dirs = map[string]*dirRecord{}
		x.dirty = false
		x.mu.Unlock()
	}
	m.closeAllLocked()
	m.open = map[string]*Index{}
	return os.RemoveAll(Dir())
}

// SizeBytes is the disk space used by the stored indexes (key excluded).
func (m *Manager) SizeBytes() int64 {
	var total int64
	entries, err := os.ReadDir(Dir())
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), indexExt) {
			if info, err := e.Info(); err == nil {
				total += info.Size()
			}
		}
	}
	return total
}
