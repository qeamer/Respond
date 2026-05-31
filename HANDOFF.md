# RESPOND+ — SYSTEM ARCHITECTURE & HANDOFF FOR CURSOR

Updated: May 31, 2026

## Terminology (required)

Do **not** name competitors or trademarked platforms in source, docs, commits, or public repos.

| Use | Avoid |
|-----|--------|
| proprietary recordings | third-party client captures |
| internal VoIP data | named legacy voice platforms |
| own datasets / custom-recorded datasets | branded training sources |

## 1. CORE PROJECT ESSENCE

Respond+ is a real-time communication and collaboration platform. The ultimate goal is a lightning-fast, native desktop application (`.exe`) for Windows built with **Go and Wails**, followed by a native Android app (`.apk`).

The core technical focus is ultra-low latency, secure tunneling, and superior voice quality.

## 2. CUSTOM VOICE ISOLATION (THE NONOISE SDK)

Crucial to the competitive edge of Respond+ is our proprietary Voice Isolation technology, **NoNoise**. We are actively developing this custom SDK (not just a standard noise gate) designed to remove handling noise, heavy keyboard clicks, breathing, and industrial/vehicle hums.

* **Official Repository:** https://github.com/Nigvar/No-noise
* **Current NoNoise Stack:** Native C++/Kotlin utilizing a custom lock-free `ring_buffer.h` and a `kissfft` Short-Time Fourier Transform (STFT) spectral pipeline.
* **ML Target:** Currently fine-tuning **DeepFilterNet3** on proprietary, custom-recorded voice and noise datasets on an RTX 5080 rig.
* **Integration Strategy:** The C++ core from the `No-noise` repository will be compiled into a cross-platform SDK. It will be hooked directly into the Respond+ backend via CGO (for Desktop) and native bindings (for Android) to process, clean, and filter audio streams before transmission.

## 3. CURRENT RESPOND+ STATUS

The project is transitioning from browser-based prototyping to a production-grade native architecture.

* **Desktop:** Wails v2 — `main.go` / `app.go` at repo root, UI in `frontend/src/index.html`, output `build/bin/Respond.exe`.
* **Backend:** Go node on `:8080` (SQLite, WebSocket `/ws`) — started inside Wails `app.startup` or via `cmd/respond-node` for browser tests.
* **Frontend:** Single-file `frontend/src/index.html` (Chat, Files, Tasks, Plus Mode).
* **Audio:** Browser mesh P2P is **removed**. Central **SFU** in Go (`internal/ws/sfu.go`).
* **Full AI changelog + file map:** see **[AI_HANDOFF.md](AI_HANDOFF.md)**.

## 4. THE ARCHITECTURAL SHIFT (CRITICAL FOR AI CONTEXT)

Do **not** waste time on browser-specific quirks, cross-origin localhost workarounds, or media autoplay hacks. Target a native **Wails (.exe)** WebView.

* **Codec:** Entire audio pipeline forced to **Opus**.
* **Pion:** `github.com/pion/webrtc/v4` terminates and routes streams on the Go node.
* **Topology:** Client holds **one** `RTCPeerConnection` to the Go node — never to other users.

```
Client (1× PC) ──Opus/WebRTC──► Go SFU (1× PC per user) ──RTP fan-out──► channel peers
                                      │
                              NoNoise hook: fanoutLoop (pkt.Payload)
```

## 5. IMPLEMENTATION NOTES (this repo)

| Area | Location |
|------|----------|
| SFU core | `internal/ws/sfu.go` |
| WS hub / signaling | `internal/ws/ws.go` |
| HTTP server | `internal/node/server.go` |
| UI (WebView) | `frontend/src/index.html` |
| Wails desktop | `main.go`, `app.go` |
| Headless / browser | `cmd/respond-node/main.go` |

Signaling events (server-only, no P2P relay): `webrtc_offer`, `webrtc_answer`, `webrtc_ice`, `webrtc_hangup`.

RTP forwarding uses `TrackLocalStaticRTP` (Opus passthrough, low latency) — not sample-based re-encode.

## 6. STRICT CODING RULES

* **Zero P2P leaks:** Signaling and voice packets flow through the central Go engine.
* **Enforce Opus:** Predictable bitrates; stages Whisper.cpp STT and NoNoise middleware.
* **CGO-friendly boundaries:** Clean RTP payload handling in `fanoutLoop` before `WriteRTP`.
* **Minimize code:** Fast, simple, no extra abstraction without need.

## 7. NEXT STEPS

1. Finish desktop login/WS (see `AI_HANDOFF.md` — `nodeHost()` fix); tray + installer.
2. NoNoise — CGO filter on RTP payload in `fanoutLoop`.
3. Optional — Whisper.cpp on the same Opus path.
