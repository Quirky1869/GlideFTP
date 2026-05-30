package keyring

import (
	"errors"

	goKeyring "github.com/zalando/go-keyring"
)

const service = "GlideFTP"

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

// IsAvailable reports whether the system keyring daemon is reachable.
func (m *Manager) IsAvailable() bool {
	_, err := goKeyring.Get(service+"_check", "_")
	return err == nil || errors.Is(err, goKeyring.ErrNotFound)
}

// Set stores a password in the keyring indexed by site ID.
func (m *Manager) Set(siteID, password string) error {
	return goKeyring.Set(service, siteID, password)
}

// Get retrieves a password from the keyring. Returns ("", nil) when not found.
func (m *Manager) Get(siteID string) (string, error) {
	pwd, err := goKeyring.Get(service, siteID)
	if errors.Is(err, goKeyring.ErrNotFound) {
		return "", nil
	}
	return pwd, err
}

// Delete removes a password from the keyring. Silently succeeds if not found.
func (m *Manager) Delete(siteID string) error {
	err := goKeyring.Delete(service, siteID)
	if errors.Is(err, goKeyring.ErrNotFound) {
		return nil
	}
	return err
}
