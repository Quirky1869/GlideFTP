package connection

import (
	"fmt"
	"sync"
)

type Status string

const (
	StatusDisconnected Status = "disconnected"
	StatusConnecting   Status = "connecting"
	StatusConnected    Status = "connected"
)

type Manager struct {
	mu     sync.Mutex
	client Client
	cfg    Config
	status Status
	cwd    string
}

func NewManager() *Manager {
	return &Manager{status: StatusDisconnected}
}

func (m *Manager) Connect(cfg Config) error {
	m.mu.Lock()
	if m.status == StatusConnecting || m.status == StatusConnected {
		m.mu.Unlock()
		return fmt.Errorf("already connected or connecting")
	}
	m.status = StatusConnecting
	m.cfg = cfg
	m.mu.Unlock()

	var client Client
	switch cfg.Protocol {
	case ProtocolSFTP:
		client = NewSFTPClient(cfg)
	default:
		client = NewFTPClient(cfg)
	}

	if err := client.Connect(); err != nil {
		m.mu.Lock()
		m.status = StatusDisconnected
		m.mu.Unlock()
		return err
	}

	cwd, _ := client.CurrentDir()
	if cwd == "" {
		cwd = "/"
	}

	m.mu.Lock()
	m.client = client
	m.status = StatusConnected
	m.cwd = cwd
	m.mu.Unlock()
	return nil
}

func (m *Manager) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.client == nil {
		m.status = StatusDisconnected
		return nil
	}
	err := m.client.Disconnect()
	m.client = nil
	m.status = StatusDisconnected
	m.cwd = ""
	return err
}

func (m *Manager) GetStatus() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *Manager) GetCwd() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cwd
}

func (m *Manager) SetCwd(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cwd = path
}

func (m *Manager) ListDir(path string) ([]RemoteFileEntry, error) {
	m.mu.Lock()
	client := m.client
	m.mu.Unlock()
	if client == nil {
		return nil, fmt.Errorf("not connected")
	}
	return client.ListDir(path)
}

func (m *Manager) MkDir(path string) error {
	m.mu.Lock()
	client := m.client
	m.mu.Unlock()
	if client == nil {
		return fmt.Errorf("not connected")
	}
	return client.MkDir(path)
}

func (m *Manager) Delete(path string) error {
	m.mu.Lock()
	client := m.client
	m.mu.Unlock()
	if client == nil {
		return fmt.Errorf("not connected")
	}
	return client.Delete(path)
}

func (m *Manager) Rename(oldPath, newPath string) error {
	m.mu.Lock()
	client := m.client
	m.mu.Unlock()
	if client == nil {
		return fmt.Errorf("not connected")
	}
	return client.Rename(oldPath, newPath)
}

func (m *Manager) GetClient() Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client
}
