package sites

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

type Protocol string
type EncryptionType string
type AuthType string

const (
	ProtocolFTP  Protocol = "ftp"
	ProtocolSFTP Protocol = "sftp"

	EncryptionNone   EncryptionType = "none"
	EncryptionTLS    EncryptionType = "tls"
	EncryptionFTPES  EncryptionType = "ftpes"

	AuthAnonymous AuthType = "anonymous"
	AuthAccount   AuthType = "account"
	AuthAskPass   AuthType = "ask_password"
	AuthInteract  AuthType = "interactive"
	AuthNormal    AuthType = "normal"
)

type Site struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Protocol   Protocol       `json:"protocol"`
	Host       string         `json:"host"`
	Port       int            `json:"port"`
	Encryption EncryptionType `json:"encryption"`
	AuthType   AuthType       `json:"authType"`
	User       string         `json:"user"`
	Password   string         `json:"password"`
	SSHKeyPath string         `json:"sshKeyPath"`
	RemoteDir  string         `json:"remoteDir"`
	Note       string         `json:"note"`
}

type Manager struct {
	sites []Site
}

func sitesPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "GlideFTP", "sites.json"), nil
}

func NewManager() *Manager {
	m := &Manager{}
	m.load()
	return m
}

func (m *Manager) load() {
	path, err := sitesPath()
	if err != nil {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	json.Unmarshal(data, &m.sites)
}

func (m *Manager) save() error {
	path, err := sitesPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	// Passwords are stored in the system keyring — never write them to disk.
	toSave := make([]Site, len(m.sites))
	for i, s := range m.sites {
		s.Password = ""
		toSave[i] = s
	}
	data, err := json.MarshalIndent(toSave, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// Persist forces a save of the current site list to disk (with passwords stripped).
func (m *Manager) Persist() error {
	return m.save()
}

func (m *Manager) GetAll() []Site {
	if m.sites == nil {
		return []Site{}
	}
	return m.sites
}

func (m *Manager) Create(s Site) (Site, error) {
	s.ID = uuid.New().String()
	m.sites = append(m.sites, s)
	return s, m.save()
}

func (m *Manager) Update(s Site) error {
	for i, existing := range m.sites {
		if existing.ID == s.ID {
			m.sites[i] = s
			return m.save()
		}
	}
	return nil
}

func (m *Manager) Delete(id string) error {
	for i, s := range m.sites {
		if s.ID == id {
			m.sites = append(m.sites[:i], m.sites[i+1:]...)
			return m.save()
		}
	}
	return nil
}

// Reorder rewrites the site list to match orderedIDs (drag-and-drop reorder
// in Site Manager) and persists it. Any existing site whose ID isn't in
// orderedIDs is kept and appended at the end, so a stale or partial list
// from the frontend can never silently drop a site.
func (m *Manager) Reorder(orderedIDs []string) error {
	byID := make(map[string]Site, len(m.sites))
	for _, s := range m.sites {
		byID[s.ID] = s
	}
	reordered := make([]Site, 0, len(m.sites))
	seen := make(map[string]bool, len(orderedIDs))
	for _, id := range orderedIDs {
		if s, ok := byID[id]; ok && !seen[id] {
			reordered = append(reordered, s)
			seen[id] = true
		}
	}
	for _, s := range m.sites {
		if !seen[s.ID] {
			reordered = append(reordered, s)
		}
	}
	m.sites = reordered
	return m.save()
}

func (m *Manager) GetByID(id string) (Site, bool) {
	for _, s := range m.sites {
		if s.ID == id {
			return s, true
		}
	}
	return Site{}, false
}
