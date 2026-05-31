// Respond+ desktop — Wails v2 shell around the Go SFU node.
//
// Build (requires Wails CLI + WebView2):
//   wails build
//
// Dev:
//   wails dev
//
// Headless node (browser / API only):
//   go run ./cmd/respond-node
package main

import (
	"io/fs"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	appfrontend "respond.app/node/frontend"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout,
		&slog.HandlerOptions{Level: slog.LevelInfo})))

	assets, err := fs.Sub(appfrontend.Assets, "src")
	if err != nil {
		slog.Error("frontend assets", "err", err)
		os.Exit(1)
	}

	app := NewApp()

	err = wails.Run(&options.App{
		Title:     "Respond",
		Width:     1280,
		Height:    720,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 30, G: 33, B: 40, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			DisableWindowIcon:  false,
		},
	})
	if err != nil {
		slog.Error("wails run failed", "err", err)
		os.Exit(1)
	}
}
