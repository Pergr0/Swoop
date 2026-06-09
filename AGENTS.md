# AGENTS.md - Swoop

Persistent project context for AI agents working in this repository. Read this
first. Keep it accurate (see "Maintenance rule" at the end).

## 1. What Swoop is

Swoop is a zero-config file-transfer app for the local network. Devices running
Swoop discover each other automatically and exchange files directly - no cloud,
no manual SMB/FTP/share setup. Desktop: Windows, macOS, Linux. Mobile today:
phones/tablets via the embedded HTTPS web UI (browser, no app install). Native
iOS/Android apps remain on the roadmap. User guide: `docs/USAGE.md`.

Core idea: do not invent an OS-level protocol and do not auto-configure the OS.
Instead ship a small cross-platform agent that speaks one open protocol over
standard TCP/UDP. This is the "protocol-first" approach (same model as LocalSend).

## 2. Requirements and principles

- **Zero-config UX**: a non-technical user opens the app, sees other devices as
  tiles (no IP/port entry), drags a file, the receiver taps "Accept". Done.
- **Speed**: transfers must be able to saturate the LAN link. Achieved via a
  dedicated data plane (parallel TCP streams, zero-copy, fast AEAD), kept
  separate from the control plane.
- **Cross-platform**: the Go core must stay platform-agnostic (stdlib + x/net
  only, no CGO). Platform specifics live in the UI/adapter layer.
- **No cloud / LAN-only** (for now): all traffic stays within the local network.
- **Security without friction**: TLS + trust-on-first-use (TOFU) with a
  fingerprint/PIN/QR confirmation for new devices.

## 3. Architecture

Protocol-first: a platform-agnostic Go core implements the protocol; the UI is
just one client of it. Control plane (HTTPS) and data plane (raw parallel TCP)
are deliberately separate so file bytes avoid HTTP overhead.

```
core/                platform-agnostic engine (no Wails imports, no CGO)
  protocol/          wire types + constants (DeviceInfo, FileMeta, ports, version)
  identity/          device id + self-signed TLS cert + fingerprint (TOFU)
  discovery/         peer discovery via UDP multicast on ALL interfaces
  transport/         control plane: HTTPS /api/v1/... (info, handshake, web upload)
  transfer/          data plane: parallel TCP streams + HTTP upload (browser)
  webpresence/       browser clients in the device grid (POST /api/v1/presence)
  webui/             embedded mobile browser page (phase 1: phone → desktop)
  staging/           directory scan + sender tree + offer root-dir summaries
  engine.go          orchestrator wiring identity + discovery + transport
app.go               Wails-bound adapter (exposes SelfInfo, Peers, events)
main.go              Wails entry point
frontend/            Svelte + TypeScript UI (device grid, live updates)
scripts/             build.ps1 (Windows), build.sh (Linux/macOS)
Makefile             thin wrappers over scripts/build.sh
```

## 4. Tech stack and toolchain

- **Core/engine**: Go (module `swoop`, Go 1.23), stdlib + `golang.org/x/net/ipv4`.
- **Desktop shell**: Wails v2 (`-tags webkit2_41` required on Linux/Ubuntu 24.04+).
- **Frontend**: Svelte + Vite + TypeScript.
- **Tooling**: Go 1.21+, Node.js LTS + npm, Wails CLI.
- **Mobile (future)**: `gomobile bind` of the same core, or native clients that
  implement the protocol.

## 5. Build and run

Use the platform build scripts; they check deps, install only what is missing,
update only what is outdated, build the Vite frontend when needed (`npm ci` on
first/outdated install, `npm run build` into `frontend/dist` for `go:embed`),
then `wails build -s` (skip Wails' duplicate frontend step). Before each build,
the scripts drop cached platform icon artifacts so the S mark always comes from
`build/appicon.png` (Windows `.ico`/`syso`, macOS `.app`/`.icns`, Linux
`build/bin/swoop.png` plus the `go:embed` in `main.go`).

```bash
# Windows
powershell -ExecutionPolicy Bypass -File scripts\build.ps1 [-CheckOnly|-Clean]
# Linux / macOS
bash scripts/build.sh [--check-only|--clean]
```

Make shortcuts: `make build`, `make check`, `make clean-deps`, `make dev`,
`make doctor`, `make core-test`, `make cross-core`, `make build-server`.

Output binary: `build/bin/swoop.exe` (Windows) or `build/bin/swoop` (Linux/mac).

Rendezvous VPS: `make build-server` (or `scripts/build-server.sh`) produces
`build/deploy/` (`swoop-rendezvous`, `install.sh`, `swoop-rendezvous.service`).
Copy to the VPS and run `sudo ./install.sh` — installs to `/opt/swoop/`,
enables `swoop-rendezvous.service`, logs to `/var/log/swoop/rendezvous.log`.
Release packaging: `make package-release` or `scripts/package-release.ps1`
(Windows) / `scripts/package-release.sh` — zips/tars `build/bin/` into
`build/release/` with `SHA256SUMS` for GitHub Releases upload.

Note: testing discovery needs TWO machines on the same LAN; two instances on one
host won't see each other (shared identity + control port).

## 6. Implemented features (current state)

- Stable device identity: ECDSA key + self-signed TLS cert, sha256 fingerprint,
  persisted under the OS config dir (`%AppData%/Swoop`, `~/.config/Swoop`).
- Peer discovery: UDP multicast announce + listen, joined and broadcast on every
  up + multicast-capable interface (or a single chosen one). Peers are addressed
  by the IP they advertise (their selected/bound interface), falling back to the
  packet source only when none is advertised — correct behind NAT/multi-adapter
  setups. `core/discovery` keeps a session-scoped first-seen order so the device
  grid does not reshuffle on poll; peers unseen for ~12s are dropped and reappear
  at the end if they return. The UI shows device name and `address:port`.
  Browser clients that open the desktop upload page also appear in the grid via
  `POST /api/v1/presence` heartbeats (`core/webpresence`): same name/address/port,
  plus a parsed User-Agent browser label; platform `web` uses the globe icon.
  Web tiles are clickable: desktop can stage and send files to the phone (phase 2
  HTTP pull). Chat is hidden for web peers (browser has no message endpoint).
- Internet invite pairing (`core/pairing`, rendezvous + overlay relay): LAN
  discovery is unchanged (peers stay visible while on the LAN; no relay slot).
  Invite-paired peers use a 20-minute idle timer after the last chat message or
  finished/canceled transfer; an active transfer (including waiting for accept)
  pauses the timer so long sends are not cut off. When idle expires, both sides
  send `POST /api/v1/goodbye` and remove the tile. The signed invite blob still
  expires in 15 minutes if nobody joins; rendezvous room TTL is refreshed on
  activity while paired. The rendezvous server scopes rooms by `sessionId`;
  overlay WebSocket attach requires the registered host or joiner `peerId`, and
  per-IP rate limits guard register/join/poll/touch/overlay endpoints.
  P2P upgrade: relay first, then QUIC; host maps QUIC UDP via UPnP when
  available; `p2pNote` on the device tile explains relay fallback. Overlay
  WebSocket uses ping keepalive; the invite host reconnects relay if the
  socket drops before a joiner arrives.
- Network interface selection: a startup picker (`core/netif` enumerates up,
  non-loopback IPv4 interfaces with a name, addresses, a kind icon
  (wifi/ethernet/tunnel/virtual/other via `frontend/src/assets/net/`), and
  best-effort link speed — Linux sysfs). The
  chosen interface determines the advertised IP and the discovery interface;
  "Auto" keeps the previous all-interface behavior. This resolves the common
  "devices see each other but transfers fail with EOF" caused by the OS auto-
  picking an unreachable adapter (e.g. a VM NAT adapter).
- Control plane: HTTPS server with `GET /api/v1/info`.
- Engine wiring + Wails app exposing `SelfInfo()` and `Peers()`, plus a live
  `peers:changed` event to the frontend.
- UI: device grid with this-device card, platform SVG icons (`frontend/src/assets/os/`),
  fingerprint, empty
  state, auto-refresh.
- File transfer (push with receiver confirmation, AirDrop-like): control-plane
  `prepare-upload` handshake that blocks for user accept/decline; data plane over
  N parallel TCP streams (default 4) with per-file range splitting; when a batch
  mixes small files with large ones (>= chunk size), at least one stream is
  reserved for large-file ranges so tiny files cannot monopolize every
  connection;
  one outgoing + one incoming session at a time; live progress (bytes, speed,
  ETA, streams). Directories (including nested) and multiple root folders are
  supported; `core/staging` scans trees, the sender UI shows an expandable
  checklist, and receivers get per-root dir counts/sizes in the offer modal.
  Files land under Downloads preserving relative paths. Control channel is TLS
  with peer-fingerprint pinning.
- Mobile browser: desktop serves `https://<ip>:<controlPort>/` (`core/webui`).
  **1:1 pairing** — QR or URL ties one browser tab to one desktop host (see
  `docs/MOBILE-WEB.md`). Phase 1 (phone → desktop): `prepare-upload` + multipart
  `POST /api/v1/upload/{session}`. Phase 2 (desktop → phone): desktop Send to
  a web peer; phone polls `GET /api/v1/pull-offer`, shows the file list, user
  accepts via `POST /api/v1/pull-offer/{session}/respond`, then downloads via
  `GET /api/v1/download/{session}/{fileId}` (one file) or `…/archive` (temp zip
  built before accept, deleted after transfer; folders / multiple files).
  Presence heartbeats keep the phone in the grid; each heartbeat returns an
  HMAC token bound to `clientId` + source IP (`core/webpresence`), required on
  pull-offer/respond/download (`X-Swoop-Web-Token`). Chat works both ways:
  browser `POST /api/v1/message` + `POST /api/v1/read`; desktop→browser via
  in-memory outbox polled at `GET /api/v1/chat?clientId&since`. Native desktop
  ↔ desktop keeps `tcp-push` with fail-closed TLS fingerprint pinning (empty
  fingerprint rejected on send and in discovery).
- Transfer UI: token-based design (`frontend/src/style.css`, readable 15px base
  type), platform SVG icons on device tiles and self-card (`assets/os/`),
  Swoop logo in the header,
  discovery status, and a
  self-device card top-right (name, address, port) plus a QR button that opens a
  modal with the mobile upload URL. Network interface is chosen
  once at startup (`StartEngine`; not hot-swappable — engine ignores re-start).
  Device grid tiles,
  compact device header, drop zone at the top of the host card, Send button
  in the transfer area, chat panel pinned to the bottom, unified progress
  component (incl. indeterminate while waiting for accept), floating cancel bar
  on the grid mid-send, incoming-offer modal with file preview and «Открыть
  папку» (`RevealDownloads` opens the OS file manager). Expandable directory
  tree with per-file/dir checkboxes,
  file/folder picker buttons, progress advanced info and Cancel (works while
  waiting for peer accept and during data transfer). Chat is collapsed by
  default behind an expander. Received files go to the user's standard OS
  Downloads folder, resolved natively per platform by `core/paths` (Windows
  known-folder API; XDG `user-dirs.dirs` on Linux, honouring localized names;
  `~/Downloads` on macOS). The path is shown in a footer (click to reveal) and
  in the receive-complete modal. The frontend cancels the webview's default
  drag-and-drop handling
  so dropped files are staged. The frontend must call runtime `OnFileDrop` (with
  `useDropTarget=false` on Windows/WebView2) to register drag listeners; the Go
  `OnFileDrop` hook in `app.go` then receives paths and emits `files:dropped`.
  The drop zone uses `--wails-drop-target: drop` for highlight feedback.
- Chat (per-peer text/links): a `POST /api/v1/message` control-plane endpoint
  (same TLS + fingerprint pinning as the rest). Messages are received without a
  prompt but are validated (UTF-8, <=8 KiB), per-peer rate-limited, and stored
  as JSON Lines in a temp file next to the binary (`swoop-chat-<pid>.tmp`) that
  is deleted on shutdown (`OnShutdown` -> `Engine.Close`); no message history is
  kept in memory. Text is treated strictly as inert data (never executed or
  interpreted; rendered as escaped text in the UI, so no script/markup/command
  injection). The chat panel lives under the drop zone in the host card, with
  per-peer unread badges on the device grid. Outgoing messages show a
  delivered tick (grey, recorded only after the receiver returns HTTP 200) that
  turns into a read tick (blue) once the peer acknowledges it: the reader posts
  a `ReadReceipt` to `POST /api/v1/read` when it opens/receives in an open chat,
  and the original sender bumps a per-peer read watermark (in memory, session-
  scoped like the chat itself) used to flag out-messages on `ChatHistory`.
  Messages carry the sender's `Ts` so both endpoints key on the same value.
  `core/chat` owns persistence; `transport.NewPinnedClient`/`VerifyFingerprint`
  is the shared pinned client.
- Production hardening: incoming offers validated (`ValidateOfferFiles`: file
  count, per-file and total size caps); `prepare-upload` JSON body capped;
  TCP data-plane chunk ranges checked against `FileMeta.Size`; control-plane
  HTTP server uses read/write/idle timeouts; `Engine.Start` marks started only
  after bind succeeds and `Engine.Close` cancels networking; peer callbacks
  (`emitPeers`) are panic-contained.
- Diagnostics: the engine writes a `swoop.log` next to the binary and mirrors
  to stderr
  covering control-plane bind/serve, prepare-upload requests/results, TLS
  handshake errors (via `http.Server.ErrorLog`), and transfer lifecycle. The
  control-plane handlers recover from panics (returning HTTP 500 instead of a
  bare connection close that a peer would see as `EOF`), and UI callbacks are
  panic-contained. Sending to self or to a peer without an address is rejected.
- Branding: the app icon source is `build/appicon.png` (1024x1024); all platforms
  refresh from it on every build. Windows: `scripts/genicon`
  regenerates `icon.ico` (gitignored) before `wails build -s -f`; scripts also
  remove `build/bin/` and any `*-res.syso` (Wails only creates `.ico` when
  missing, otherwise the default W mark sticks). macOS:
  Wails packs `iconfile.icns` into `swoop.app`; `build.sh` deletes any cached
  `build/darwin/iconfile.icns` and the previous `swoop.app` first. Linux: PNG
  is `go:embed` in `main.go` -> `linux.Options.Icon` (GTK window/taskbar);
  `build.sh` drops stale `build/bin/swoop.png` then copies a fresh one plus
  `install-desktop-entry.sh` for the application-menu icon.
  The mark is a white "S" formed
  by two opposing arrows (right = share, left = copy/receive) on the app's
  blue->green gradient (`#3A8BFF`->`#3DDC84`), reading as bidirectional transfer.
  The app was renamed from "Shapy" to "Swoop" because "Shapy" collided with
  existing products (a fitness app on the App Store/Play, the `shapy.app` CSS
  tool). The "S" monogram still fits the new name. If renamed again, the mark is
  name-agnostic and survives it.
- Cross-platform build scripts (Windows + Linux/macOS) with dependency
  check/install/update, optional clean, ASCII-only output, LF endings enforced
  via `.gitattributes`. They also auto-reinstall `frontend/node_modules` when it
  was built on another OS (platform-specific esbuild), via a `.swoop-platform`
  marker.

## 7. Not yet implemented (roadmap)

- Data-plane hardening: per-file sha256 integrity, resumable transfers, and AEAD
  encryption of the data channel (currently token-authenticated plaintext on LAN).
- Trust/pairing UI (explicit TOFU confirmation, PIN/QR); the control channel
  already pins the peer fingerprint.
- mDNS/DNS-SD layered on top of the multicast fallback.
- QR / manual-IP fallback for networks with client isolation.
- Windows firewall prompt/rule on first run.
- Mobile browser polish: resumable `Range` pull, WebSocket progress on the phone.
- Native mobile apps (iOS/Android app store builds).

## 8. Conventions

- Keep `core/` free of UI/Wails imports and CGO so it cross-compiles and can be
  reused by mobile later. Verify with `make cross-core`.
- Control plane = HTTPS (small/control messages); data plane = raw TCP (bytes).
- Shell scripts and Makefile use LF endings and plain ASCII output only.
- After substantive edits, run `go vet ./core/...`. Test builds (`make build`,
  the scripts, `npm run build`) are fine while implementing/debugging a
  feature, but remove artifacts before finishing: `build/bin`, generated
  `build/windows/icon.ico`, `frontend/dist` (keep `.gitkeep`), and
  `frontend/node_modules`. The maintainer does release builds elsewhere.

## 9. Maintenance rule (IMPORTANT)

This file is the source of truth for project context. Whenever you make a change
that affects any of the following, update AGENTS.md in the SAME change:

- a new feature or component is added, or an existing one is removed/renamed;
- requirements, goals, or principles change;
- the architecture, package layout, or tech stack changes;
- build/run steps, scripts, or toolchain requirements change;
- an item moves between "Implemented features" (section 6) and "Roadmap"
  (section 7).

Concretely: when you finish a feature, move it from section 7 to section 6 and
adjust sections 3-5 if the structure changed. Keep entries short and accurate.
Do not let this file drift from the actual code.
