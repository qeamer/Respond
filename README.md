<p align="center">
  <img src="frontend/src/assets/respond-logo.png" alt="Respond" width="220">
</p>

# Respond+

**Respond — lynrask teamkommunikasjon med tale, chat og filer på noden.**

Real-time voice and collaboration: native **Go + Wails** desktop (Windows), with optional browser testing via the headless node.

**Repo:** [github.com/qeamer/Respond](https://github.com/qeamer/Respond)

## Stack

| Layer | Technology |
|-------|------------|
| Backend | Go HTTP + WebSocket on `:8080`, SQLite |
| Voice | Central **SFU** ([Pion WebRTC v4](https://github.com/pion/webrtc)) — **Opus**, server-mediated |
| UI | `frontend/src/index.html` embedded via `//go:embed` |

## Requirements

- Go 1.24+
- Wails v2 + WebView2 (desktop build)

## Build and run

### Desktop (Wails)

```powershell
cd respond-v2
go mod tidy
wails dev
wails build
```

Output: `build/bin/Respond.exe`. The local node listens on `:8080`; the UI uses `ws://127.0.0.1:8080/ws`.

### Headless node (browser)

```powershell
go build -ldflags="-s -w" -o respond-node.exe ./cmd/respond-node
.\respond-node.exe
```

Open [http://localhost:8080](http://localhost:8080). Dev WS auth: `dev:<userId>:<displayName>`.

## Voice (SFU)

```
Client (1× RTCPeerConnection) ──WebRTC──► Go SFU ──Opus RTP fan-out──► channel peers
```

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
