package main

// Update check: asks GitHub for the latest published release (drafts and
// pre-releases are never returned by /releases/latest) and compares its tag
// with appVersion. Nothing is downloaded or installed here - the frontend shows
// a banner with a direct link to the right file for this install (the .exe on
// Windows, the matching AppImage / .deb / .rpm / archive on Linux, the yay
// command for the AUR package) and a link to the release page.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	localfs "GlideFTP/internal/fs"
)

const (
	updateRepo       = "Quirky1869/GlideFTP"
	updateCheckURL   = "https://api.github.com/repos/" + updateRepo + "/releases/latest"
	updateTimeoutSec = 10
)

// UpdateInfo is what CheckForUpdate returns to the frontend.
type UpdateInfo struct {
	Current    string `json:"current"`
	Latest     string `json:"latest"`
	Available  bool   `json:"available"`
	ReleaseURL string `json:"releaseUrl"`
	// Kind tells the frontend which wording to use:
	// exe | appimage | deb | rpm | aur | linux.
	Kind string `json:"kind"`
	// DownloadURL is the direct link to the file for this install ("" when the
	// release has no such asset, or for the AUR: Command is used instead).
	DownloadURL  string `json:"downloadUrl"`
	DownloadName string `json:"downloadName"`
	Command      string `json:"command"`
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// CheckForUpdate fetches the latest GitHub release. An error (no network,
// proxy, GitHub rate limit) is only shown when the user asked for the check.
func (a *App) CheckForUpdate() (*UpdateInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), updateTimeoutSec*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, updateCheckURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "GlideFTP/"+appVersion) // required by the GitHub API

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub: HTTP %d", resp.StatusCode)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	latest := strings.TrimPrefix(rel.TagName, "v")
	newer, err := isNewerVersion(latest, appVersion)
	if err != nil {
		return nil, err
	}

	info := &UpdateInfo{
		Current:    appVersion,
		Latest:     latest,
		Available:  newer,
		ReleaseURL: rel.HTMLURL,
	}
	info.Kind, info.DownloadName, info.Command = installKind(latest)
	for _, asset := range rel.Assets {
		if info.DownloadName != "" && asset.Name == info.DownloadName {
			info.DownloadURL = asset.URL
		}
	}
	return info, nil
}

// OpenUpdateLink opens a release / download link in the default browser.
// Only links to this project's GitHub releases are accepted.
func (a *App) OpenUpdateLink(url string) error {
	if !strings.HasPrefix(url, "https://github.com/"+updateRepo+"/releases/") {
		return errors.New("refused: not a GlideFTP release link")
	}
	return localfs.OpenURL(url)
}

// installKind works out how this copy of GlideFTP was installed, and so which
// release file updates it. Asset names match create-archive.sh / release.yml.
func installKind(ver string) (kind, asset, command string) {
	if goruntime.GOOS == "windows" {
		return "exe", "GlideFTP-Windows-v" + ver + ".exe", ""
	}
	// AppImage runtime sets $APPIMAGE to the .AppImage file's path.
	if img := os.Getenv("APPIMAGE"); img != "" {
		variant := "Debian"
		if strings.Contains(filepath.Base(img), "Arch") {
			variant = "Arch"
		}
		return "appimage", "GlideFTP-Linux-" + variant + "-AppImage-v" + ver + ".tar.gz", ""
	}
	// Packages (AUR, .deb, .rpm) all install the binary as /usr/bin/glideftp.
	if exe, err := os.Executable(); err == nil && exe == "/usr/bin/glideftp" {
		switch {
		case fileExists("/etc/arch-release"):
			return "aur", "", "yay -S glideftp-bin"
		case fileExists("/etc/debian_version"):
			return "deb", "GlideFTP-Linux-v" + ver + ".deb", ""
		case fileExists("/etc/fedora-release"), fileExists("/etc/redhat-release"):
			return "rpm", "GlideFTP-Linux-v" + ver + ".rpm", ""
		}
	}
	return "linux", "GlideFTP-Linux-v" + ver + ".tar.gz", ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isNewerVersion reports whether latest (X.Y.Z) is above current (X.Y.Z),
// number by number: 1.10.0 is newer than 1.9.0.
func isNewerVersion(latest, current string) (bool, error) {
	l, err := parseVersion(latest)
	if err != nil {
		return false, err
	}
	c, err := parseVersion(current)
	if err != nil {
		return false, err
	}
	for i := range l {
		if l[i] != c[i] {
			return l[i] > c[i], nil
		}
	}
	return false, nil
}

func parseVersion(v string) ([3]int, error) {
	var out [3]int
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, fmt.Errorf("invalid version %q", v)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, fmt.Errorf("invalid version %q", v)
		}
		out[i] = n
	}
	return out, nil
}
