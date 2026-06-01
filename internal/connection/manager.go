package connection

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusDisconnected Status = "disconnected"
	StatusConnecting   Status = "connecting"
	StatusConnected    Status = "connected"
)

type connEntry struct {
	id     string
	name   string
	client Client
	cfg    Config
	cwd    string
}

type Manager struct {
	mu           sync.Mutex
	conns        []*connEntry
	activeID     string
	isConnecting bool
	onLost       func(id, host string)
}

func NewManager() *Manager {
	return &Manager{}
}

// SetOnConnectionLost registers a callback invoked when a keepalive detects that
// a connection was dropped unexpectedly by the remote server.
func (m *Manager) SetOnConnectionLost(fn func(id, host string)) {
	m.mu.Lock()
	m.onLost = fn
	m.mu.Unlock()
}

// getActive returns the active entry. Caller must hold m.mu.
func (m *Manager) getActive() *connEntry {
	for _, c := range m.conns {
		if c.id == m.activeID {
			return c
		}
	}
	return nil
}

// getActiveClient returns the active client. Caller must hold m.mu.
func (m *Manager) getActiveClient() Client {
	if c := m.getActive(); c != nil {
		return c.client
	}
	return nil
}

func removeEntries(conns []*connEntry, id string) []*connEntry {
	var result []*connEntry
	for _, c := range conns {
		if c.id != id {
			result = append(result, c)
		}
	}
	return result
}

// Connect establishes a connection, replacing the currently active one.
// Other connections (if any) remain open. Returns the new connection ID.
func (m *Manager) Connect(cfg Config, name string) (string, error) {
	if name == "" {
		name = cfg.Host
	}

	m.mu.Lock()
	if m.isConnecting {
		m.mu.Unlock()
		return "", fmt.Errorf("already connecting, please wait")
	}
	// Grab and remove the active connection entry.
	active := m.getActive()
	var oldClient Client
	if active != nil {
		oldClient = active.client
		m.conns = removeEntries(m.conns, active.id)
	}
	m.isConnecting = true
	m.mu.Unlock()

	if oldClient != nil {
		go oldClient.Disconnect()
	}

	id, err := m.doConnect(cfg, name)

	m.mu.Lock()
	m.isConnecting = false
	if err != nil {
		m.mu.Unlock()
		return "", err
	}
	m.activeID = id
	m.mu.Unlock()
	return id, nil
}

// ConnectNew adds a new connection alongside existing ones.
// The new connection becomes the active one. Returns the new connection ID.
func (m *Manager) ConnectNew(cfg Config, name string) (string, error) {
	if name == "" {
		name = cfg.Host
	}

	m.mu.Lock()
	if m.isConnecting {
		m.mu.Unlock()
		return "", fmt.Errorf("already connecting, please wait")
	}
	m.isConnecting = true
	m.mu.Unlock()

	id, err := m.doConnect(cfg, name)

	m.mu.Lock()
	m.isConnecting = false
	if err != nil {
		m.mu.Unlock()
		return "", err
	}
	m.activeID = id
	m.mu.Unlock()
	return id, nil
}

// doConnect creates the client, connects, and appends the entry to m.conns.
// Must NOT be called with m.mu held.
func (m *Manager) doConnect(cfg Config, name string) (string, error) {
	var client Client
	switch cfg.Protocol {
	case ProtocolSFTP:
		client = NewSFTPClient(cfg)
	default:
		client = NewFTPClient(cfg)
	}

	if err := client.Connect(); err != nil {
		return "", err
	}

	cwd, _ := client.CurrentDir()
	if cwd == "" {
		cwd = "/"
	}

	id := uuid.New().String()
	entry := &connEntry{
		id:     id,
		name:   name,
		client: client,
		cfg:    cfg,
		cwd:    cwd,
	}

	m.mu.Lock()
	m.conns = append(m.conns, entry)
	m.mu.Unlock()

	m.startKeepalive(entry)
	return id, nil
}

// startKeepalive runs a goroutine that pings the connection every 60 seconds.
// If the ping fails the connection is removed from the manager and onLost is called.
func (m *Manager) startKeepalive(entry *connEntry) {
	id := entry.id
	client := entry.client
	host := entry.cfg.Host

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			m.mu.Lock()
			stillPresent := false
			for _, c := range m.conns {
				if c.id == id {
					stillPresent = true
					break
				}
			}
			onLost := m.onLost
			m.mu.Unlock()

			if !stillPresent {
				return // connection was closed normally
			}

			if err := client.Keepalive(); err != nil {
				m.mu.Lock()
				before := len(m.conns)
				m.conns = removeEntries(m.conns, id)
				removed := len(m.conns) < before
				if m.activeID == id {
					m.activeID = ""
					if len(m.conns) > 0 {
						m.activeID = m.conns[len(m.conns)-1].id
					}
				}
				m.mu.Unlock()

				if removed && onLost != nil {
					onLost(id, host)
				}
				return
			}
		}
	}()
}

// Disconnect closes ALL connections.
func (m *Manager) Disconnect() error {
	m.mu.Lock()
	conns := m.conns
	m.conns = nil
	m.activeID = ""
	m.mu.Unlock()

	var lastErr error
	for _, c := range conns {
		if c.client != nil {
			if err := c.client.Disconnect(); err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}

// CloseOne closes a specific connection by ID.
// If it was the active connection, the most-recently-added remaining one becomes active.
func (m *Manager) CloseOne(id string) error {
	m.mu.Lock()
	var found *connEntry
	for _, c := range m.conns {
		if c.id == id {
			found = c
			break
		}
	}
	if found == nil {
		m.mu.Unlock()
		return fmt.Errorf("connection not found")
	}
	m.conns = removeEntries(m.conns, id)
	if m.activeID == id {
		if len(m.conns) > 0 {
			m.activeID = m.conns[len(m.conns)-1].id
		} else {
			m.activeID = ""
		}
	}
	m.mu.Unlock()

	if found.client != nil {
		return found.client.Disconnect()
	}
	return nil
}

// SwitchTo makes a connection the active one (does not reconnect).
func (m *Manager) SwitchTo(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.conns {
		if c.id == id {
			m.activeID = id
			return nil
		}
	}
	return fmt.Errorf("connection not found: %s", id)
}

// GetConnections returns info about all active connections.
func (m *Manager) GetConnections() []ConnInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	infos := make([]ConnInfo, 0, len(m.conns))
	for _, c := range m.conns {
		infos = append(infos, ConnInfo{
			ID:       c.id,
			Name:     c.name,
			Host:     c.cfg.Host,
			Protocol: string(c.cfg.Protocol),
			Port:     c.cfg.Port,
			User:     c.cfg.User,
		})
	}
	return infos
}

// ActiveID returns the ID of the currently active connection.
func (m *Manager) ActiveID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeID
}

// ConnectionCount returns the total number of open connections.
func (m *Manager) ConnectionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.conns)
}

func (m *Manager) GetStatus() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.isConnecting {
		return StatusConnecting
	}
	if m.activeID != "" && len(m.conns) > 0 {
		return StatusConnected
	}
	return StatusDisconnected
}

func (m *Manager) GetCwd() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c := m.getActive(); c != nil {
		return c.cwd
	}
	return ""
}

func (m *Manager) SetCwd(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c := m.getActive(); c != nil {
		c.cwd = path
	}
}

func (m *Manager) GetClient() Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getActiveClient()
}

func (m *Manager) GetActiveHost() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c := m.getActive(); c != nil {
		return c.cfg.Host
	}
	return ""
}

func (m *Manager) ListDir(path string) ([]RemoteFileEntry, error) {
	m.mu.Lock()
	active := m.getActive()
	if active == nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("not connected")
	}
	client := active.client
	entryID := active.id
	timeout := active.cfg.TimeoutSec
	m.mu.Unlock()

	if client == nil {
		return nil, fmt.Errorf("not connected")
	}
	if timeout <= 0 {
		timeout = 30
	}

	type res struct {
		entries []RemoteFileEntry
		err     error
	}
	ch := make(chan res, 1)
	go func() {
		entries, err := client.ListDir(path)
		ch <- res{entries, err}
	}()

	select {
	case r := <-ch:
		return r.entries, r.err
	case <-time.After(time.Duration(timeout) * time.Second):
		m.mu.Lock()
		for _, c := range m.conns {
			if c.id == entryID && c.client == client {
				go c.client.Disconnect()
				c.client = nil
				m.conns = removeEntries(m.conns, entryID)
				if m.activeID == entryID {
					m.activeID = ""
					if len(m.conns) > 0 {
						m.activeID = m.conns[len(m.conns)-1].id
					}
				}
				break
			}
		}
		m.mu.Unlock()
		return nil, fmt.Errorf("operation timed out, please reconnect")
	}
}

func (m *Manager) MkDir(path string) error {
	m.mu.Lock()
	client := m.getActiveClient()
	m.mu.Unlock()
	if client == nil {
		return fmt.Errorf("not connected")
	}
	return client.MkDir(path)
}

func (m *Manager) Delete(path string) error {
	m.mu.Lock()
	client := m.getActiveClient()
	m.mu.Unlock()
	if client == nil {
		return fmt.Errorf("not connected")
	}
	return client.Delete(path)
}

func (m *Manager) Rename(oldPath, newPath string) error {
	m.mu.Lock()
	client := m.getActiveClient()
	m.mu.Unlock()
	if client == nil {
		return fmt.Errorf("not connected")
	}
	return client.Rename(oldPath, newPath)
}

// CopyRemote copies a single remote file by downloading to a temp file then re-uploading.
// Works for both FTP and SFTP without requiring a server-side copy command.
func (m *Manager) CopyRemote(srcPath, destPath string) error {
	m.mu.Lock()
	client := m.getActiveClient()
	m.mu.Unlock()
	if client == nil {
		return fmt.Errorf("not connected")
	}

	tmp, err := os.CreateTemp("", "glideftp-rcopy-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	ctx := context.Background()
	if err := client.Download(ctx, srcPath, tmpPath, nil); err != nil {
		return err
	}
	return client.Upload(ctx, tmpPath, destPath, nil)
}

// CopyRemoteDir recursively copies a remote directory tree.
// Runs synchronously so the caller can await completion and refresh the panel.
func (m *Manager) CopyRemoteDir(srcPath, destPath string) error {
	if err := m.MkDir(destPath); err != nil {
		return err
	}
	entries, err := m.ListDir(srcPath)
	if err != nil {
		return err
	}
	for _, e := range entries {
		dst := destPath + "/" + e.Name
		if e.IsDir {
			if err := m.CopyRemoteDir(e.Path, dst); err != nil {
				return err
			}
		} else {
			if err := m.CopyRemote(e.Path, dst); err != nil {
				return err
			}
		}
	}
	return nil
}
