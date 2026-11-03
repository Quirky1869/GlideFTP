package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Settings struct {
	Theme                          string `json:"theme"`
	Language                       string `json:"language"`
	MaxConcurrentTransfers         int    `json:"maxConcurrentTransfers"`
	DefaultLocalDir                string `json:"defaultLocalDir"`
	DefaultPort                    int    `json:"defaultPort"`
	ConnectionTimeoutSec           int    `json:"connectionTimeoutSec"`
	ShowHiddenFiles                bool   `json:"showHiddenFiles"`
	ConfirmOnDelete                bool   `json:"confirmOnDelete"`
	DateFormat                     string `json:"dateFormat"`
	MaxTransferSpeedKBps           int    `json:"maxTransferSpeedKBps"`
	AccentColor                    string `json:"accentColor"`
	AccentGradient                 bool   `json:"accentGradient"`
	AccentColor2                   string `json:"accentColor2"`
	MaxConnections                 int    `json:"maxConnections"`
	ConnectCardShadow              bool   `json:"connectCardShadow"`
	WindowWidth                    int    `json:"windowWidth"`
	WindowHeight                   int    `json:"windowHeight"`
	StartMaximized                 bool   `json:"startMaximized"`
	ShowWindowSizeBadge            bool   `json:"showWindowSizeBadge"` // "W × H" badge while resizing (WindowSizeOverlay)
	CloseSiteManagerOnClickOutside bool   `json:"closeSiteManagerOnClickOutside"`
	DoubleClickNavigateUp          bool   `json:"doubleClickNavigateUp"`
	NotificationSoundEnabled       bool   `json:"notificationSoundEnabled"`
	NotificationSound              string `json:"notificationSound"`
	// In-app bubble (bottom right) when the whole transfer queue is finished.
	QueueDoneBubble bool `json:"queueDoneBubble"`
	// Last command line chosen for "Run a command" after the queue (pre-filled
	// the next time). Only this one string is kept.
	QueueDoneCommand string `json:"queueDoneCommand"`
	// Original file name of the user-supplied sound (NotificationSound "custom"),
	// the file itself lives in the config dir (see app_sound.go). "" = none.
	CustomSoundName string `json:"customSoundName"`
	// Remember listed remote folders to speed up searches (opt-in, see
	// internal/searchindex). SearchIndexAsked: the first-launch explanation
	// popup has been answered on this device.
	SearchIndexEnabled bool `json:"searchIndexEnabled"`
	SearchIndexAsked   bool `json:"searchIndexAsked"`
	// Look for a newer GitHub release at startup (app_update.go).
	// SkippedUpdateVersion: "Ignorer cette version" was clicked for it, no
	// banner until a newer one is published.
	CheckUpdates         bool   `json:"checkUpdates"`
	SkippedUpdateVersion string `json:"skippedUpdateVersion"`
	// Synchronized browsing (Ctrl+Y): create the folder missing on the
	// other side without asking (the dialog's "create and don't ask again").
	SyncCreateMissing bool `json:"syncCreateMissing"`
	// Panel comparison (Ctrl+O): criterion "size" or "date", and whether
	// identical files are hidden while it is on.
	CompareBy            string `json:"compareBy"`
	CompareHideIdentical bool   `json:"compareHideIdentical"`
	// Align the rows of both panels (gaps where a name is missing), like FileZilla.
	CompareAlign bool `json:"compareAlign"`
	// Sort file names character by character like FileZilla (dot names,
	// then UPPER CASE, then lower case) instead of the explorer order.
	CaseSensitiveSort bool `json:"caseSensitiveSort"`
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
		ConfirmOnDelete:                true,
		DateFormat:                     "%yyyy-%MM-%dd %hh:%mm",
		MaxTransferSpeedKBps:           0,
		AccentColor:                    "#5B8AF5",
		AccentGradient:                 false,
		AccentColor2:                   "#C15BF5",
		MaxConnections:                 3,
		ConnectCardShadow:              false,
		WindowWidth:                    1400,
		WindowHeight:                   900,
		StartMaximized:                 false,
		ShowWindowSizeBadge:            true,
		CloseSiteManagerOnClickOutside: true,
		DoubleClickNavigateUp:          false,
		NotificationSoundEnabled:       false,
		NotificationSound:              "chime",
		SearchIndexEnabled:             false,
		SearchIndexAsked:               false,
		CheckUpdates:                   true,
		CompareBy:                      "size",
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
	// Pre-v1.7.8 files hold an unused Go layout ("2006-01-02 15:04"); the
	// frontend now expects %-tokens (see frontend/src/utils/dateFormat.js).
	if s.DateFormat != "system" && !strings.Contains(s.DateFormat, "%") {
		s.DateFormat = Default().DateFormat
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
