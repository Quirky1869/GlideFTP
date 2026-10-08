//go:build linux

package fs

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

// openerGrace is how long we wait for the opener to fail. xdg-open either
// exits quickly (gio/kde-open hand over to the app, or report "no
// application") or - in its generic mode (window managers like Hyprland
// without a full desktop) - runs the application itself in the foreground and
// only returns when it closes. Still running after this delay = launched.
const openerGrace = 2 * time.Second

// OpenWithDefault opens path (file or folder) with the desktop's default
// application, via xdg-open.
func OpenWithDefault(path string) error {
	if isGenericDesktop() && !hasDefaultApp(path) {
		return ErrNoDefaultApp
	}
	err := startAndCheck("xdg-open", path)
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		// xdg-open exit codes: 3 = no tool/application found, 4 = action failed.
		if code := exitErr.ExitCode(); code == 3 || code == 4 {
			return ErrNoDefaultApp
		}
	}
	return err
}

// fullDesktops are the XDG_CURRENT_DESKTOP values for which xdg-open hands
// the file to the desktop's own opener (gio open, kde-open, exo-open...).
// Those resolve parent types (a shell script opens in the text editor) and
// fail properly - or show their own "Open with" - when nothing fits.
var fullDesktops = toSet(
	"GNOME", "GNOME-FLASHBACK", "KDE", "XFCE", "LXDE", "LXQT", "MATE",
	"CINNAMON", "X-CINNAMON", "ENLIGHTENMENT", "DEEPIN", "UKUI", "BUDGIE",
)

// isGenericDesktop reports whether xdg-open will run in its "generic" mode
// (window managers / compositors such as Hyprland, sway, i3). In that mode,
// with no application for the file type, xdg-open silently falls back to a
// web browser and still exits 0 - so we check the association ourselves.
func isGenericDesktop() bool {
	for _, d := range strings.Split(os.Getenv("XDG_CURRENT_DESKTOP"), ":") {
		if fullDesktops[strings.ToUpper(strings.TrimSpace(d))] {
			return false
		}
	}
	if os.Getenv("KDE_FULL_SESSION") != "" || os.Getenv("GNOME_DESKTOP_SESSION_ID") != "" {
		return false
	}
	return true
}

// hasDefaultApp asks xdg-mime - which reads the same mimeapps.list /
// defaults.list / mimeinfo.cache files as xdg-open's generic mode - whether
// an application is associated with path's MIME type. When xdg-mime itself
// is unusable, it answers true and lets xdg-open decide.
func hasDefaultApp(path string) bool {
	mime, err := xdgMime("query", "filetype", path)
	if err != nil || mime == "" {
		return true
	}
	app, err := xdgMime("query", "default", mime)
	if err != nil {
		return true
	}
	return app != ""
}

func xdgMime(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "xdg-mime", args...)
	cmd.Env = cleanEnv(os.Environ())
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// OpenWithApp launches appPath with filePath as its only argument.
func OpenWithApp(appPath, filePath string) error {
	return startAndCheck(appPath, filePath)
}

// startAndCheck starts the command detached from the UI and reports an error
// only if it exits with a failure within openerGrace. The process is always
// reaped (cmd.Wait in a goroutine) so no zombie is left behind.
func startAndCheck(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Env = cleanEnv(os.Environ())
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(openerGrace):
		return nil
	}
}

// Variables an AppImage runtime / AppRun injects to make GlideFTP find its
// bundled libraries. An external application inheriting them could load the
// AppImage's libraries instead of its own and crash.
var appImageVars = toSet(
	"APPIMAGE", "APPDIR", "ARGV0", "OWD",
	"LD_LIBRARY_PATH", "LD_PRELOAD", "WEBKIT_EXEC_PATH",
	"GDK_PIXBUF_MODULE_FILE", "GDK_PIXBUF_MODULEDIR",
	"GIO_MODULE_DIR", "GIO_EXTRA_MODULES", "GSETTINGS_SCHEMA_DIR",
	"GTK_PATH", "GTK_EXE_PREFIX", "GTK_DATA_PREFIX", "GTK_IM_MODULE_FILE",
	"QT_PLUGIN_PATH", "PYTHONPATH", "PYTHONHOME", "PERLLIB",
	"GST_PLUGIN_SYSTEM_PATH", "GST_PLUGIN_SYSTEM_PATH_1_0",
	"GST_PLUGIN_PATH", "GST_PLUGIN_PATH_1_0",
	"GST_PLUGIN_SCANNER", "GST_PLUGIN_SCANNER_1_0",
)

func toSet(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

// cleanEnv returns env unchanged outside an AppImage. Inside one (APPIMAGE or
// APPDIR set), it drops the variables above and, for every other variable,
// removes the path entries pointing inside the AppImage mount (e.g. the
// $APPDIR/usr/bin added to PATH or $APPDIR/usr/share to XDG_DATA_DIRS),
// dropping the variable if nothing is left.
func cleanEnv(env []string) []string {
	appDir := os.Getenv("APPDIR")
	if appDir == "" && os.Getenv("APPIMAGE") == "" {
		return env
	}
	out := make([]string, 0, len(env))
	for _, kv := range env {
		name, value, ok := strings.Cut(kv, "=")
		if !ok || appImageVars[name] {
			continue
		}
		if appDir != "" && strings.Contains(value, appDir) {
			var kept []string
			for _, part := range strings.Split(value, ":") {
				if !strings.HasPrefix(part, appDir) {
					kept = append(kept, part)
				}
			}
			if len(kept) == 0 {
				continue
			}
			value = strings.Join(kept, ":")
		}
		out = append(out, name+"="+value)
	}
	return out
}
