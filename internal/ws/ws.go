// Package ws implements the WebSocket hub for Respond Node.
// Handles connections, chat, presence, and centralized WebRTC SFU signaling.
package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"respond.app/node/internal/db"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 45 * time.Second
	maxMessageSize = 32 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(*http.Request) bool { return true },
}

// Envelope is the JSON wrapper for all WebSocket events.
type Envelope struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data,omitempty"`
}

// Client represents one WebSocket session.
type Client struct {
	UserID    string
	Username  string
	Role      string
	ChannelID string
	Speaking  bool
	send      chan []byte
}

func (c *Client) Send(env Envelope) {
	b, _ := json.Marshal(env)
	select {
	case c.send <- b:
	default:
		// queue full — drop
	}
}

// Hub manages all connected clients and the voice SFU.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]*Client
	sfu     *SFU
	polls   *PollManager
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]*Client),
		sfu:     newSFU(),
		polls:   newPollManager(),
	}
}

func (h *Hub) ActiveCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	if old, ok := h.clients[c.UserID]; ok && old != c {
		// Same user reconnected from a new tab — close the old session.
		close(old.send)
	}
	h.clients[c.UserID] = c
	h.mu.Unlock()
	h.BroadcastPresence()
}

func (h *Hub) Unregister(userID string, c *Client) {
	h.mu.Lock()
	cur, ok := h.clients[userID]
	removed := ok && cur == c
	if removed {
		delete(h.clients, userID)
		close(c.send)
	}
	h.mu.Unlock()
	if removed {
		c.Speaking = false
		h.BroadcastPresence()
	}
}

func (h *Hub) RelayToUser(userID string, env Envelope) bool {
	h.mu.RLock()
	c, ok := h.clients[userID]
	h.mu.RUnlock()
	if ok {
		c.Send(env)
	}
	return ok
}

func (h *Hub) BroadcastToChannel(channelID, excludeID string, env Envelope) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.clients {
		if c.UserID == excludeID || c.ChannelID != channelID {
			continue
		}
		c.Send(env)
	}
}

func (h *Hub) ChannelPeers(callerID, channelID string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var peers []string
	for _, c := range h.clients {
		if c.UserID != callerID && c.ChannelID == channelID {
			peers = append(peers, c.UserID)
		}
	}
	return peers
}

func (h *Hub) BroadcastPresence() {
	h.mu.RLock()
	type p struct {
		ID        string `json:"id"`
		Username  string `json:"username"`
		ChannelID string `json:"channel_id"`
		Speaking  bool   `json:"speaking"`
	}
	users := make([]p, 0, len(h.clients))
	for _, c := range h.clients {
		users = append(users, p{c.UserID, c.Username, c.ChannelID, c.Speaking})
	}
	clients := make([]*Client, 0, len(h.clients))
	for _, c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	data, _ := json.Marshal(map[string]any{"users": users})
	env := Envelope{Event: "presence_update", Data: data}
	for _, c := range clients {
		c.Send(env)
	}
}

// Handler upgrades HTTP to WebSocket.
func Handler(hub *Hub, database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Warn("ws: upgrade failed", "addr", r.RemoteAddr, "err", err)
			return
		}
		slog.Info("ws: upgrade ok", "addr", r.RemoteAddr)

		// First message: hello
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		var hello Envelope
		if err := conn.ReadJSON(&hello); err != nil || hello.Event != "hello" {
			slog.Warn("ws: bad or missing hello", "addr", r.RemoteAddr, "err", err, "event", hello.Event)
			conn.Close()
			return
		}
		conn.SetReadDeadline(time.Time{})

		// Parse token: "dev:<userid>:<username>"
		var h struct{ Token string `json:"token"` }
		json.Unmarshal(hello.Data, &h)

		c := &Client{
			ChannelID: "lobby",
			send:      make(chan []byte, 64),
		}
		if strings.HasPrefix(h.Token, "dev:") {
			parts := strings.SplitN(h.Token, ":", 3)
			if len(parts) == 3 && parts[1] != "" {
				c.UserID, c.Username, c.Role = parts[1], parts[2], "guest"
			}
		}
		if c.UserID == "" {
			// Reject connections without valid token — prevents anonymous ghost users
			slog.Warn("ws: rejected connection without valid token", "addr", r.RemoteAddr)
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(4001, "authentication required"))
			conn.Close()
			return
		}

		database.UpsertUser(c.UserID, c.Username, c.Role)
		slog.Info("ws: connected", "user", c.UserID, "name", c.Username)

		hub.Register(c)
		defer func() {
			hub.sfu.RemovePeer(c.UserID)
			data, _ := json.Marshal(map[string]string{"user_id": c.UserID})
			hub.BroadcastToChannel(c.ChannelID, c.UserID,
				Envelope{Event: "peer_left", Data: data})
			hub.Unregister(c.UserID, c)
			slog.Info("ws: disconnected", "user", c.UserID)
		}()

		go writePump(conn, c)
		readPump(conn, c, hub, database)
	}
}

func readPump(conn *websocket.Conn, c *Client, hub *Hub, database *db.DB) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("readPump panic", "user", c.UserID, "err", r)
		}
		conn.Close()
	}()
	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		var env Envelope
		if err := conn.ReadJSON(&env); err != nil {
			return
		}
		if env.Event == "" || env.Event == "ping" {
			continue
		}
		switch env.Event {
		case "join_channel":
			handleJoin(c, hub, database, env.Data)
		case "chat_message":
			handleChat(c, hub, database, env.Data)
		case "webrtc_offer":
			hub.sfu.HandleOffer(c, env.Data)
		case "webrtc_answer":
			hub.sfu.HandleAnswer(c, env.Data)
		case "webrtc_ice":
			hub.sfu.HandleICE(c, env.Data)
		case "webrtc_hangup":
			hub.sfu.RemovePeer(c.UserID)
		case "speaking_state":
			handleSpeakingState(c, hub, env.Data)
		case "poll_vote":
			hub.handlePollVote(c, env.Data)
		}
	}
}

func handleSpeakingState(c *Client, hub *Hub, data json.RawMessage) {
	var req struct {
		Speaking bool `json:"speaking"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		return
	}
	c.Speaking = req.Speaking
	hub.BroadcastPresence()
}

func writePump(conn *websocket.Conn, c *Client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func handleJoin(c *Client, hub *Hub, database *db.DB, data json.RawMessage) {
	var req struct{ ChannelID string `json:"channel_id"` }
	if json.Unmarshal(data, &req); req.ChannelID == "" {
		return
	}
	if c.ChannelID != "" && c.ChannelID != req.ChannelID {
		hub.sfu.OnChannelChange(c.UserID)
		c.Speaking = false
	}
	c.ChannelID = req.ChannelID
	hub.sfu.UpdateMemberChannel(c.UserID, req.ChannelID)
	hub.BroadcastPresence()
	hub.broadcastPollUpdate(req.ChannelID)
	// Send last 50 messages
	msgs, _ := database.RecentMessages(req.ChannelID, 50)
	data2, _ := json.Marshal(map[string]any{"messages": msgs})
	c.Send(Envelope{Event: "sync_state", Data: data2})
}

func handleChat(c *Client, hub *Hub, database *db.DB, data json.RawMessage) {
	var req struct {
		ChannelID string `json:"channel_id"`
		Content   string `json:"content"`
		MsgType   string `json:"type"`
	}
	if err := json.Unmarshal(data, &req); err != nil || req.Content == "" || req.ChannelID == "" {
		return
	}
	if req.MsgType == "" {
		req.MsgType = "chat"
	}
	database.InsertMessage(req.ChannelID, c.UserID, c.Username, req.Content, req.MsgType)
	out, _ := json.Marshal(map[string]any{
		"channel_id": req.ChannelID,
		"user_id":    c.UserID,
		"username":   c.Username,
		"content":    req.Content,
		"type":       req.MsgType,
	})
	hub.BroadcastToChannel(req.ChannelID, c.UserID,
		Envelope{Event: "chat_message", Data: out})
}

