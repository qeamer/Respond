// Package db handles SQLite storage for Respond Node.
// Pure Go, no CGo — uses modernc.org/sqlite via mattn/go-sqlite3 driver.
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct{ *sql.DB }

func New(path string) (*DB, error) {
	d, err := sql.Open("sqlite", path+"?_journal=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(1) // SQLite likes single writer
	if err := d.Ping(); err != nil {
		return nil, err
	}
	return &DB{d}, nil
}

func (d *DB) Migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			role TEXT NOT NULL DEFAULT 'guest',
			created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
			last_seen INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS channels (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			password_hash TEXT,
			is_ghost INTEGER NOT NULL DEFAULT 0,
			ord INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			channel_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			username TEXT NOT NULL,
			content TEXT NOT NULL,
			msg_type TEXT NOT NULL DEFAULT 'chat',
			created_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_msg_channel_time
			ON messages(channel_id, created_at DESC)`,
	}
	for _, s := range stmts {
		if _, err := d.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	// Seed default channels
	defaults := []struct {
		id, name string
		ord      int
		ghost    bool
	}{
		{"lobby", "Lobby", 0, false},
		{"spill1", "Spill.1", 10, false},
		{"spill2", "Spill.2", 20, false},
		{"spill3", "Spill.3", 30, false},
		{"spill4", "Spill.4", 40, false},
		{"spill5", "Spill.5", 50, false},
		{"spill6", "Spill.6", 60, false},
		{"mote1", "Møte.1", 70, false},
		{"ghost1", "Ghost.1", 80, true},
		{"ghost2", "Ghost.2", 90, true},
		{"afk", "afk", 999, false},
	}
	for _, c := range defaults {
		_, err := d.Exec(`INSERT OR IGNORE INTO channels (id, name, is_ghost, ord) VALUES (?, ?, ?, ?)`,
			c.id, c.name, boolToInt(c.ghost), c.ord)
		if err != nil {
			return err
		}
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ── Messages ───────────────────────────────────────────────────────

type Message struct {
	ID        int64
	ChannelID string
	UserID    string
	Username  string
	Content   string
	MsgType   string
	CreatedAt int64
}

func (d *DB) InsertMessage(channelID, userID, username, content, msgType string) (*Message, error) {
	res, err := d.Exec(
		`INSERT INTO messages (channel_id, user_id, username, content, msg_type) VALUES (?, ?, ?, ?, ?)`,
		channelID, userID, username, content, msgType)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Message{ID: id, ChannelID: channelID, UserID: userID,
		Username: username, Content: content, MsgType: msgType, CreatedAt: time.Now().Unix()}, nil
}

func (d *DB) RecentMessages(channelID string, limit int) ([]Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := d.Query(
		`SELECT id, channel_id, user_id, username, content, msg_type, created_at
		 FROM messages WHERE channel_id = ? ORDER BY created_at DESC LIMIT ?`,
		channelID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ChannelID, &m.UserID, &m.Username,
			&m.Content, &m.MsgType, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	// reverse for chronological order
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, rows.Err()
}

// ── Users ──────────────────────────────────────────────────────────

func (d *DB) UpsertUser(id, username, role string) error {
	_, err := d.Exec(
		`INSERT INTO users (id, username, role, last_seen) VALUES (?, ?, ?, strftime('%s','now'))
		 ON CONFLICT(id) DO UPDATE SET username = excluded.username, last_seen = strftime('%s','now')`,
		id, username, role)
	return err
}

var ErrNotFound = errors.New("not found")
