# Respond+

Real-time communication and collaboration platform targeting a native **Go + Wails** desktop app (Windows `.exe`) and later Android (`.apk`). Focus: ultra-low latency voice, secure tunneling, and proprietary **NoNoise** voice isolation.

**Upstream:** [github.com/qeamer/Respond](https://github.com/qeamer/Respond.git)

## Stack (this tree)

| Layer | Technology |
|-------|------------|
| Backend | Go HTTP + WebSocket on `:8080`, SQLite (`modernc.org/sqlite`) |
| Voice | Central **SFU** via [Pion WebRTC v4](https://github.com/pion/webrtc) — **Opus only**, no browser mesh P2P |
| UI | Embedded `client.html` (`//go:embed`) — prototype shell for Wails WebView |
| Future | NoNoise C++ SDK via CGO in RTP fan-out path; Whisper.cpp STT |

## Requirements

- Go 1.24+ ([go.dev/dl](https://go.dev/dl/))
- No CGo required for SQLite (pure Go driver)

## Build and run

```powershell
cd respond-v2
go mod tidy
go build -ldflags="-s -w" -o respond.exe ./cmd/respond
.\respond.exe
```

Open [http://localhost:8080](http://localhost:8080). WebSocket endpoint: `ws://localhost:8080/ws`.

Or use `start.bat` (builds and opens the browser). **Shipping target is a Wails desktop app** — browser is prototype only. See **[ROADMAP.md](ROADMAP.md)**.

Dev auth token format (first WS message): `dev:<userId>:<displayName>`.

## Voice architecture (SFU)

```
Client (1× RTCPeerConnection) ──WebRTC──► Go SFU (1× PC per user)
                                              │
                    Opus RTP fan-out ◄──────────┘ per channel/lobby
```

- Clients send `webrtc_offer` / `webrtc_answer` / `webrtc_ice` **only to the server** (no `to` peer field).
- Server forwards each publisher's Opus RTP to every other member in the same channel.
- Hook point for **NoNoise**: `internal/ws/sfu.go` — `fanoutLoop` before `WriteRTP`.
- Changing channel tears down voice (`webrtc_hangup` / server `RemovePeer`) and re-joins after `join_channel`.

## Project layout

```
cmd/respond/     main.go, embedded client.html
internal/db/       SQLite persistence
internal/ws/     WebSocket hub + SFU (ws.go, sfu.go)
```

Wails binding will live beside `cmd/respond` (e.g. `app.go`) and call the same `Hub` / `SFU` types.

## Handoff for Cursor / contributors

See **[HANDOFF.md](HANDOFF.md)** for full architecture, terminology rules (no third-party platform names in repo), and integration points.

## Related repositories

- **NoNoise SDK:** [github.com/Nigvar/No-noise](https://github.com/Nigvar/No-noise) — STFT / ring-buffer pipeline; ML trained on proprietary custom-recorded datasets; integrate at SFU RTP buffer.

## License

MIT — see [LICENSE](LICENSE).
