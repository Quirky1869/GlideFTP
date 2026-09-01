package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Settings struct {
	Theme                          string `json:"theme"`
	Language                       string `json:"language"`
	MaxConcurrentTransfers         int    `json:"maxConcurrentTransfers"`
	DefaultLocalDir                string `json:"defaultLocalDir"`
	DefaultPort                    int    `json:"defaultPort"`
	ConnectionTimeoutSec           int    `json:"connectionTimeoutSec"`
	ShowHiddenFiles                bool   `json:"showHiddenFiles"`
	PassiveMode                    bool   `json:"passiveMode"`
	AutoReconnect                  bool   `json:"autoReconnect"`
	ConfirmOnDelete                bool   `json:"confirmOnDelete"`
	DateFormat                     string `json:"dateFormat"`
	MaxTransferSpeedKBps           int    `json:"maxTransferSpeedKBps"`
	AccentColor                    string `json:"accentColor"`
	MaxConnections                 int    `json:"maxConnections"`
	ConnectCardShadow              bool   `json:"connectCardShadow"`
	WindowWidth                    int    `json:"windowWidth"`
	WindowHeight                   int    `json:"windowHeight"`
	StartMaximized                 bool   `json:"startMaximized"`
	CloseSiteManagerOnClickOutside bool   `json:"closeSiteManagerOnClickOutside"`
	DoubleClickNavigateUp          bool   `json:"doubleClickNavigateUp"`
	NotificationSoundEnabled       bool   `json:"notificationSoundEnabled"`
	NotificationSound              string `json:"notificationSound"`
}

func Default() *Settings {
	home, _ := os.UserHomeDir()
	return &Settings{
		Theme:                          "dark",
		Language:                       "en",
		MaxConcurrentTransfers:         3,
		DefaultLocalDir:                home,
		DefaultPort:                    21,
		ConnectionTimeoutSec:           60,
		ShowHiddenFiles:                false,
		PassiveMode:                    true,
		AutoReconnect:                  false,
		ConfirmOnDelete:                true,
		DateFormat:                     "2006-01-02 15:04",
		MaxTransferSpeedKBps:           0,
		AccentColor:                    "#5B8AF5",
		MaxConnections:                 3,
		ConnectCardShadow:              false,
		WindowWidth:                    1400,
		WindowHeight:                   900,
		StartMaximized:                 false,
		CloseSiteManagerOnClickOutside: true,
		DoubleClickNavigateUp:          false,
		NotificationSoundEnabled:       false,
		NotificationSound:              "chime",
	}
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "GlideFTP", "settings.json"), nil
}

func Load() (*Settings, error) {
	path, err := configPath()
	if err != nil {
		return Default(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Default(), nil
	}
	s := Default()
	if err := json.Unmarshal(data, s); err != nil {
		return Default(), nil
	}
	return s, nil
}

func (s *Settings) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
