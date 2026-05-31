// Package node runs the local HTTP + WebSocket + SFU signaling server.
package node

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"respond.app/node/internal/db"
	"respond.app/node/internal/ws"
)

// Config configures the local node HTTP server.
type Config struct {
	Addr    string
	DBPath  string
	ServeUI func() ([]byte, error) // optional; nil = API + WS only (Wails desktop)
}

// Server wraps SQLite, the WS hub, and the HTTP listener.
type Server struct {
	cfg    Config
	db     *db.DB
	hub    *ws.Hub
	http   *http.Server
}

// New opens the database, migrates, and prepares the HTTP mux (does not listen yet).
func New(cfg Config) (*Server, error) {
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "respond.db"
	}

	database, err := db.New(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	if err := database.Migrate(); err != nil {
		database.Close()
		return nil, err
	}

	hub := ws.NewHub()
	mux := http.NewServeMux()

	if cfg.ServeUI != nil {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" && r.URL.Path != "/index.html" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			data, err := cfg.ServeUI()
			if err != nil {
				http.Error(w, "ui not found", http.StatusInternalServerError)
				return
			}
			w.Write(data)
		})
	}

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

	return &Server{
		cfg: cfg,
		db:  database,
		hub: hub,
		http: &http.Server{
			Addr:    cfg.Addr,
			Handler: mux,
		},
	}, nil
}

// Hub returns the WebSocket hub (for tests or future bindings).
func (s *Server) Hub() *ws.Hub {
	return s.hub
}

// Start listens in a goroutine. Safe to call once after New.
func (s *Server) Start() {
	go func() {
		abs, _ := filepath.Abs(s.cfg.DBPath)
		slog.Info("Respond Node HTTP+WebSocket listening",
			"addr", s.cfg.Addr,
			"db", abs,
			"url", "http://127.0.0.1"+s.cfg.Addr)
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
		}
	}()
}

// Shutdown stops the HTTP server and closes the database.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.http != nil {
		_ = s.http.Shutdown(ctx)
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// DefaultDBPath returns a per-user database path for desktop installs.
func DefaultDBPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "respond.db", nil
	}
	appDir := filepath.Join(dir, "Respond")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "respond.db", nil
	}
	return filepath.Join(appDir, "respond.db"), nil
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
	_ = json.NewEncoder(w).Encode(v)
}

// ShutdownTimeout is the default graceful shutdown window.
const ShutdownTimeout = 5 * time.Second
