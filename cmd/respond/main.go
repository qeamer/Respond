// Respond Node — single-binary version with SQLite.
//
// Build for current platform:
//   go build -ldflags="-s -w" -o respond ./cmd/respond
//
// Run:
//   ./respond
//
// Open browser to http://localhost:8080
package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"respond.app/node/internal/db"
	"respond.app/node/internal/ws"
)

//go:embed client.html
var staticFiles embed.FS

func main() {
	addr := flag.String("addr", ":8080", "HTTP+WS address")
	dbPath := flag.String("db", "respond.db", "SQLite database path")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout,
		&slog.HandlerOptions{Level: slog.LevelInfo})))

	abs, _ := filepath.Abs(*dbPath)
	slog.Info("Respond Node starting", "addr", *addr, "db", abs)

	database, err := db.New(*dbPath)
	if err != nil {
		slog.Error("db open failed", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		slog.Error("migrate failed", "err", err)
		os.Exit(1)
	}
	slog.Info("database ready")

	hub := ws.NewHub()

	mux := http.NewServeMux()

	// Serve embedded HTML at /
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		data, _ := staticFiles.ReadFile("client.html")
		w.Write(data)
	})

	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/api/v1/node/health", cors(func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, map[string]any{
			"node":            "Respond-Node",
			"connected_users": hub.ActiveCount(),
			"version":         "1.0-sqlite",
		})
	}))

	mux.HandleFunc("/ws", ws.Handler(hub, database))

	srv := &http.Server{
		Addr:    *addr,
		Handler: mux,
	}

	go func() {
		slog.Info("HTTP+WebSocket listening", "url", "http://localhost"+*addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

func cors(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		h(w, r)
	}
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
