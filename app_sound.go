package main

// Custom notification sound: the user picks an audio file, the frontend checks
// it can be decoded, then it is copied to <config>/GlideFTP/custom-sound.<ext>.
// The frontend plays it from a data: URI - WebKitGTK's <audio> can't load the
// wails:// scheme (see frontend/src/utils/sound.js). Playback is capped at
// 5 seconds by the frontend, whatever the file's length.

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Base64 inflates by 1/3 and the whole file goes to the WebView - a 5 s clip is
// well under this even as WAV, a bigger file is almost certainly not a short sound.
const customSoundMaxBytes = 10 << 20

var customSoundMimes = map[string]string{
	".mp3":  "audio/mpeg",
	".wav":  "audio/wav",
	".ogg":  "audio/ogg",
	".oga":  "audio/ogg",
	".opus": "audio/ogg",
	".flac": "audio/flac",
	".m4a":  "audio/mp4",
	".aac":  "audio/aac",
	".webm": "audio/webm",
}

// CustomSoundFile is a picked (not yet installed) sound: Path is passed back to
// InstallCustomSound once the frontend has checked DataURL plays.
type CustomSoundFile struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	DataURL string `json:"dataUrl"`
}

func customSoundDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "GlideFTP"), nil
}

// installedCustomSound returns the path of the installed custom sound, "" if none.
func installedCustomSound() string {
	dir, err := customSoundDir()
	if err != nil {
		return ""
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "custom-sound.*"))
	for _, m := range matches {
		if _, ok := customSoundMimes[strings.ToLower(filepath.Ext(m))]; ok {
			return m
		}
	}
	return ""
}

func soundDataURL(path string) (string, error) {
	mime, ok := customSoundMimes[strings.ToLower(filepath.Ext(path))]
	if !ok {
		return "", errors.New("unsupported_format")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Size() > customSoundMaxBytes {
		return "", errors.New("too_large")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// ChooseCustomSound opens a file picker for an audio file. Returns nil when cancelled.
// Error strings recognised by the frontend: unsupported_format, too_large.
func (a *App) ChooseCustomSound() (*CustomSoundFile, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Notification sound",
		Filters: []runtime.FileFilter{
			{DisplayName: "Audio (mp3, wav, ogg, opus, flac, m4a, aac, webm)", Pattern: "*.mp3;*.wav;*.ogg;*.oga;*.opus;*.flac;*.m4a;*.aac;*.webm"},
		},
	})
	if err != nil || path == "" {
		return nil, err
	}
	url, err := soundDataURL(path)
	if err != nil {
		return nil, err
	}
	return &CustomSoundFile{Path: path, Name: filepath.Base(path), DataURL: url}, nil
}

// InstallCustomSound copies the picked file to the config dir, replacing any
// previous custom sound. Returns the original file name (shown in Settings).
func (a *App) InstallCustomSound(path string) (string, error) {
	if _, err := soundDataURL(path); err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	dir, err := customSoundDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	removeCustomSoundFiles(dir)
	dest := filepath.Join(dir, "custom-sound"+strings.ToLower(filepath.Ext(path)))
	if err := os.WriteFile(dest, data, 0600); err != nil {
		return "", fmt.Errorf("write custom sound: %w", err)
	}
	return filepath.Base(path), nil
}

// GetCustomSound returns the installed custom sound as a data: URI, "" if none.
func (a *App) GetCustomSound() string {
	path := installedCustomSound()
	if path == "" {
		return ""
	}
	url, err := soundDataURL(path)
	if err != nil {
		return ""
	}
	return url
}

// RemoveCustomSound deletes the installed custom sound file.
func (a *App) RemoveCustomSound() error {
	dir, err := customSoundDir()
	if err != nil {
		return err
	}
	removeCustomSoundFiles(dir)
	return nil
}

func removeCustomSoundFiles(dir string) {
	matches, _ := filepath.Glob(filepath.Join(dir, "custom-sound.*"))
	for _, m := range matches {
		_ = os.Remove(m)
	}
}
