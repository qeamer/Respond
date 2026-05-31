// Respond Node — headless HTTP + WebSocket server (browser prototype).
//
//   go build -o respond.exe ./cmd/respond-node
//   ./respond.exe
//
// Open http://localhost:8080
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	appfrontend "respond.app/node/frontend"
	"respond.app/node/internal/node"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP+WS address")
	dbPath := flag.String("db", "respond.db", "SQLite database path")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout,
		&slog.HandlerOptions{Level: slog.LevelInfo})))

	abs, _ := filepath.Abs(*dbPath)
	slog.Info("Respond Node starting", "addr", *addr, "db", abs)

	srv, err := node.New(node.Config{
		Addr:   *addr,
		DBPath: *dbPath,
		ServeUI: func() ([]byte, error) {
			return appfrontend.Assets.ReadFile("src/index.html")
		},
	})
	if err != nil {
		slog.Error("node init failed", "err", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), node.ShutdownTimeout)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	srv.Start()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	slog.Info("shutting down")
}
