# Respond+ — Roadmap

Updated: May 31, 2026

## Milestone A — Done (this repo)

- Go node on `:8080` with SQLite, WebSocket, embedded UI
- Central SFU (Pion), Opus-only, single `RTCPeerConnection` per client
- PTT / Voice Activation, channel voice, multi-browser proof
- Browser UI is a **prototype shell** only — not the shipping product

## Milestone B — Desktop app (next, ~2–4 weeks)

**Goal:** Installable `Respond.exe` via **Wails v2**, same backend code, no browser testing dependency.

| Step | Work |
|------|------|
| B1 | ~~Wails scaffold~~ — `cmd/respond` + `frontend/src/index.html` + `wails.json` (done) |
| B2 | System tray, single instance, auto-start node in-process (no separate port confusion) |
| B3 | Native audio: WASAPI capture/playback or keep WebRTC in WebView with `wails://` origin |
| B4 | Settings persist to `%AppData%/Respond/` |
| B5 | Installer (NSIS or WiX) + signed build pipeline |

**Why Wails:** Reuse current UI and Go SFU; one repo; fast path to “real program on PC.”

## Milestone C — NoNoise on the node

| Step | Work |
|------|------|
| C1 | CGO link NoNoise C++ SDK into `fanoutLoop` (and optional uplink preprocess) |
| C2 | Settings: NoNoise enabled (replace greyed placeholder) |
| C3 | Tune latency vs quality for gaming voice |

## Milestone D — Video (webcam + screen)

| Step | Work |
|------|------|
| D1 | SFU video tracks (VP8/AV1 or H.264) — fan-out like audio |
| D2 | Simulcast / quality tiers for screen share |
| D3 | Match clarity targets (low latency, sharp text on share) |

## Milestone E — Android

**Goal:** Simple app — login, channel list, PTT/VA, minimal UI.

| Step | Work |
|------|------|
| E1 | Kotlin + WebRTC client OR Flutter with same WS/SFU protocol |
| E2 | NoNoise via JNI (same C++ core as desktop) |
| E3 | Background voice, Bluetooth headset, low battery mode |
| E4 | Push notifications for mentions (later) |

Protocol stays identical: `dev:` token → WS → `join_channel` → `webrtc_offer` to node.

## What to stop doing

- Relying on Chrome/Firefox/Vivaldi quirks as the main test path
- Adding features only in browser without Wails parity

## Suggested order for you

1. **Push & tag** this tree on GitHub (`v0.21-sfu-browser-prototype`)
2. **Wails shell** — open app, connect, voice in one window
3. **NoNoise** hook when Ingvar SDK is ready
4. **Android** proof-of-concept against same node

See [HANDOFF.md](HANDOFF.md) for architecture and terminology rules.
