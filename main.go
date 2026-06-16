package main

import (
	"embed"

	"GlideFTP/internal/settings"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	s, _ := settings.Load()
	w, h := s.WindowWidth, s.WindowHeight
	if w <= 0 {
		w = 1400
	}
	if h <= 0 {
		h = 900
	}

	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "GlideFTP",
		Width:            w,
		Height:           h,
		MinWidth:         900,
		MinHeight:        600,
		MaxWidth:         0,
		MaxHeight:        0,
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 23, A: 1},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind:       []interface{}{app},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
