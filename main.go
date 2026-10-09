package main

import (
	"embed"
	"runtime"

	"GlideFTP/internal/settings"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// linuxMaxWindowSize is passed as MaxWidth/MaxHeight on Linux instead of 0.
// Wails' GTK backend (internal/frontend/desktop/linux/window.c SetMinMaxSize)
// replaces a 0 max with the geometry of the monitor the window opens on and
// sends it as a GDK_HINT_MAX_SIZE geometry hint. Compositors that enforce
// size hints (Hyprland) then cap the window - even maximized/fullscreen - at
// that monitor's size: opened on a 1920x1200 laptop screen, it can't fill a
// larger ultrawide. A large explicit max avoids the substitution. Linux only:
// Windows treats 0 as "no limit" already.
const linuxMaxWindowSize = 16384

func main() {
	s, _ := settings.Load()
	w, h := s.WindowWidth, s.WindowHeight
	if w <= 0 {
		w = 1400
	}
	if h <= 0 {
		h = 900
	}

	maxW, maxH := 0, 0
	if runtime.GOOS == "linux" {
		maxW, maxH = linuxMaxWindowSize, linuxMaxWindowSize
	}

	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "GlideFTP",
		Width:            w,
		Height:           h,
		MinWidth:         900,
		MinHeight:        600,
		MaxWidth:         maxW,
		MaxHeight:        maxH,
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 23, A: 1},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		// Warns about opened files with changes not sent back (app_openfile.go).
		OnBeforeClose: app.beforeClose,
		// One GlideFTP per user session: a second launch just brings the
		// running window back (D-Bus on Linux, a named mutex on Windows).
		// Keeps two instances from writing the same search index / settings.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "com.quirky1869.glideftp",
			OnSecondInstanceLaunch: app.onSecondInstanceLaunch,
		},
		Bind: []interface{}{app},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
