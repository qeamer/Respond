package main

import (
	"context"
	"log/slog"
	"os"

	"respond.app/node/internal/node"
)

// App is the Wails lifecycle bridge; starts the local node (Hub/SFU) for the WebView UI.
type App struct {
	node *node.Server
}

// NewApp constructs the application bindings struct.
func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout,
		&slog.HandlerOptions{Level: slog.LevelInfo})))

	dbPath, err := node.DefaultDBPath()
	if err != nil {
		slog.Warn("using default db path", "err", err)
		dbPath = "respond.db"
	}

	srv, err := node.New(node.Config{
		Addr:   ":8080",
		DBPath: dbPath,
	})
	if err != nil {
		slog.Error("node init failed", "err", err)
		return
	}

	a.node = srv
	srv.Start()
	slog.Info("Respond desktop ready", "ws", "ws://127.0.0.1:8080/ws")
}

func (a *App) shutdown(ctx context.Context) {
	if a.node == nil {
		return
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, node.ShutdownTimeout)
	defer cancel()
	if err := a.node.Shutdown(shutdownCtx); err != nil {
		slog.Error("node shutdown", "err", err)
	}
	slog.Info("Respond desktop stopped")
}
