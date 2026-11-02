// Package favorites keeps the favourite folders and files of each panel in
// ~/.config/GlideFTP/favorites.json: one list for the local disk ("local",
// shared by every server) and one per server account (key built by the
// frontend from protocol, user, host and port).
package favorites

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Favorite struct {
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

type Manager struct {
	mu    sync.Mutex
	lists map[string][]Favorite
}

func filePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "GlideFTP", "favorites.json"), nil
}

func NewManager() *Manager {
	m := &Manager{lists: map[string][]Favorite{}}
	if path, err := filePath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			json.Unmarshal(data, &m.lists)
		}
	}
	if m.lists == nil {
		m.lists = map[string][]Favorite{}
	}
	return m
}

func (m *Manager) All() map[string][]Favorite {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string][]Favorite, len(m.lists))
	for k, v := range m.lists {
		out[k] = append([]Favorite(nil), v...)
	}
	return out
}

// Set replaces the list of one key; an empty list removes the key.
func (m *Manager) Set(key string, list []Favorite) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(list) == 0 {
		delete(m.lists, key)
	} else {
		m.lists[key] = append([]Favorite(nil), list...)
	}
	path, err := filePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m.lists, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
