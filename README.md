<p align="center">
  <img src="frontend/src/assets/respond-logo.png" alt="Respond" width="220">
</p>

# Respond+

**Respond — lynrask teamkommunikasjon med tale, chat og filer på noden.**

Respond+ is a real-time voice, chat, and collaboration app built around a local node. The desktop app uses **Go + Wails** on Windows, while the backend runs a Go HTTP/WebSocket server with a central WebRTC SFU.

**Repo:** [github.com/qeamer/Respond](https://github.com/qeamer/Respond)

## What It Does

- Runs a local Respond node on `:8080`.
- Serves a Wails desktop UI and an optional browser UI for testing.
- Routes voice through a central **Pion WebRTC SFU**: each client connects to the node, and the node forwards Opus RTP to peers in the same channel.
- Stores users and chat history in SQLite.

## Stack

| Layer | Technology |
|-------|------------|
| Backend | Go HTTP + WebSocket on `:8080`, SQLite |
| Voice | Central **SFU** ([Pion WebRTC v4](https://github.com/pion/webrtc)) — **Opus**, server-mediated |
| UI | `frontend/src/index.html` embedded via `//go:embed` |

## Local Setup

### 1. Verify Requirements

```powershell
go version
```

Success: prints `go version go1.24... windows/amd64` or newer.

```powershell
wails version
```

Success: prints Wails `v2.x`.

```powershell
git --version
```

Success: prints Git `2.x`.

WebView2 is also required for the desktop app:

```powershell
Test-Path "${env:ProgramFiles(x86)}\Microsoft\EdgeWebView\Application"
```

Success: prints `True`.

### 2. Clone And Install Dependencies

```powershell
git clone https://github.com/qeamer/Respond.git respond-v2
Set-Location respond-v2
go mod tidy
```

Success: command exits without errors.

```powershell
go test ./...
```

Success: all packages pass.

### 3. Build Desktop App

```powershell
wails build
```

Verify:

```powershell
Test-Path .\build\bin\Respond.exe
```

Success: prints `True`.

### 4. Run Desktop App

```powershell
.\build\bin\Respond.exe
```

Verify node health in a second terminal:

```powershell
(Invoke-WebRequest -UseBasicParsing http://127.0.0.1:8080/api/v1/node/health).Content
```

Success: returns JSON with `node`, `connected_users`, and `version`.

### Headless node (browser)

```powershell
go build -ldflags="-s -w" -o respond-node.exe ./cmd/respond-node
.\respond-node.exe
```

Verify:

```powershell
(Invoke-WebRequest -UseBasicParsing http://127.0.0.1:8080/api/v1/node/health).Content
```

Success: returns node health JSON.

Open [http://localhost:8080](http://localhost:8080).

## Common Pitfalls

### 1. Port `8080` Can Only Have One Owner

`Respond.exe` starts the local node internally. `respond-node.exe` also tries to bind `:8080`. Do not run both unless one is stopped first.

```powershell
netstat -ano | Select-String ":8080"
```

Success: one `LISTENING` process at most.

### 2. Frontend Changes Require A Rebuild

`frontend/src/index.html` is embedded into the executable at build time.

```powershell
wails build
.\build\bin\Respond.exe
```

Success: the built app shows your latest UI changes.

### 3. WebSocket Auth Is Still Dev-Only

The current local prototype uses `dev:<userId>:<displayName>` for WebSocket login. This is for development only and must be replaced before exposing a node outside a trusted network.

### 4. Voice Is SFU-Based, Not Peer-To-Peer

Debug audio in `internal/ws/sfu.go` and the client WebRTC path. Clients do not connect directly to each other.

### 5. Chat History Is Sent On Channel Join

`internal/db/db.go` stores messages. `internal/ws/ws.go` calls `RecentMessages(channel, 50)` during `join_channel` and sends `sync_state`.

## Voice (SFU)

```
Client (1× RTCPeerConnection) ──WebRTC──► Go SFU ──Opus RTP fan-out──► channel peers
```

## Example Change: Add A WebSocket Event

This example adds a `typing_indicator` event.

### 1. Add Server Handler

In `internal/ws/ws.go`, add a case in `readPump`:

```go
case "typing_indicator":
	handleTyping(c, hub, env.Data)
```

Add the handler:

```go
func handleTyping(c *Client, hub *Hub, data json.RawMessage) {
	var req struct {
		IsTyping bool `json:"is_typing"`
	}
	if err := json.Unmarshal(data, &req); err != nil || c.ChannelID == "" {
		return
	}
	out, err := json.Marshal(map[string]any{
		"user_id":   c.UserID,
		"is_typing": req.IsTyping,
	})
	if err != nil {
		return
	}
	hub.BroadcastToChannel(c.ChannelID, c.UserID, Envelope{Event: "typing_indicator", Data: out})
}
```

Verify:

```powershell
go test ./internal/ws
```

Success: package tests pass.

### 2. Add Frontend Send/Receive

Edit `frontend/src/index.html` to send the new event and handle `typing_indicator` in the WebSocket event dispatcher.

Verify JavaScript syntax:

```powershell
node -e "const s=require('fs').readFileSync('frontend/src/index.html','utf8').match(/<script>([\s\S]*)<\/script>/)[1];require('fs').writeFileSync('_check_index.js',s)"
node --check _check_index.js
Remove-Item _check_index.js
```

Success: `node --check` prints no syntax errors.

### 3. Build And Test With Two Clients

```powershell
wails build
.\build\bin\Respond.exe
```

In a browser, open [http://localhost:8080](http://localhost:8080) as a second client.

Verify: both clients can join the same channel and receive the new event.

## Layout

```
main.go, app.go       Wails entry
cmd/respond-node/     Headless server
frontend/src/         UI + assets
internal/ws/          WebSocket hub + SFU
internal/node/        HTTP server
```

## License

MIT — see [LICENSE](LICENSE).
