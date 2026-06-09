# Swoop

Zero-config LAN and internet file transfer for Windows, macOS, and Linux. Phones via browser. No cloud accounts.

> **Quick start (Russian):** [docs/USAGE.md](docs/USAGE.md)

## How to use

### Desktop ↔ desktop (LAN)

1. Launch Swoop and pick a network interface (Wi‑Fi / Ethernet).
2. Click a device tile in the grid.
3. Drag files into the window (or use the file/folder picker), then **Send**.
4. The receiver accepts or declines in the incoming-offer dialog. Files land in
   the OS **Downloads** folder.
5. Optional: expand **Messages** under the drop zone for short text/links.

LAN peers stay visible while discovery packets arrive (~every 3 s). There is no
idle timeout — only the usual “device went offline” when it leaves the network or
closes the app.

### Desktop ↔ desktop (internet)

For two machines in different places (no shared Wi‑Fi), use a signed
**SwoopInvite** — the same files and chat as on LAN, routed through a small
rendezvous/relay for signaling (with optional direct P2P upgrade when NAT allows).

| Step | Host (creates invite) | Joiner (imports invite) |
|------|------------------------|-------------------------|
| 1 | On the self-card, open **Connect over the internet** (globe). | **Import invite** — pick the `.swoopinvite` file or a QR image. |
| 2 | Share the QR or invite file out-of-band (messengers, email). | Accept the invite before it expires. |
| 3 | When the joiner appears in the grid, send files or chat as on LAN. | Same — drag, **Send**, **Messages**. |

**Timers**

- **15 minutes** — invite window: both sides must finish pairing in this time,
  or the host must create a new invite.
- **20 minutes idle** — after pairing, if there is no chat and no active file
  transfer (including waiting for Accept), both sides send goodbye and the tile
  disappears. Long transfers are not cut off — the idle timer pauses until the
  transfer ends.

**Tile badges** (subtitle on each device): **L** = local LAN, **I** = internet
invite; for internet peers also **R** (relay) or **P2P** (direct QUIC when
upgrade succeeds). Compare the fingerprint on first connect (TOFU).

Internet pairing is desktop-only today; the mobile browser path remains LAN-only.

### Phone / tablet (browser)

The desktop serves a minimal web UI at `https://<desktop-ip>:53317/`. Pairing
is **1:1** (QR on the self-card, or open the URL manually). Phone and PC must
share the same LAN.

| Direction | What you do |
|-----------|-------------|
| **Phone → PC** | On the phone page: pick files → Send. Confirm on the desktop. |
| **PC → phone** | On the desktop: select the browser peer → Send. On the phone: **Accept and download** (multi-file/folder → one `.zip`). |
| **Chat** | «Сообщения» on the phone page; **Messages** on the desktop when the web peer is selected. |

First visit: accept the self-signed TLS warning and compare the short
fingerprint shown on both sides (TOFU).

More detail: [docs/MOBILE-WEB.md](docs/MOBILE-WEB.md) · user guide (RU):
[docs/USAGE.md](docs/USAGE.md)

## Language

The desktop app and the phone browser page use **Russian** when the OS (or
browser) locale list includes Russian (`ru`, `ru-RU`, …). Any other locale
gets **English**. There is no in-app language switch.

To preview English on a Russian macOS system, temporarily reorder languages
(quit Swoop first, restore when done):

```bash
# save current order
defaults read -g AppleLanguages

# English first (restart Swoop after)
defaults write -g AppleLanguages -array "en-US" "ru-RU"

# restore Russian (use your saved values)
defaults write -g AppleLanguages -array "ru-RU" "en-RU"
```

On the phone, move **English** above Russian in system language settings and
reopen the Swoop page in the browser. See [docs/USAGE.md](docs/USAGE.md) (RU)
for the same steps in Russian.

## Architecture (short)

Protocol-first: a platform-agnostic Go core (`core/`) implements the protocol,
and the UI is just one client of it.

- `core/protocol` — wire types and constants (one source of truth)
- `core/identity` — device id + self-signed TLS cert + fingerprint (TOFU trust)
- `core/discovery` — peer discovery via UDP multicast on all interfaces
- `core/invite` + `core/pairing` — signed SwoopInvite blobs and paired-peer registry
- `core/rendezvous` + `core/overlay` — internet signaling and invite-scoped relay
  (optional QUIC P2P upgrade)
- `core/transport` — control plane (HTTPS `/api/v1/...`: info, handshake, goodbye)
- `core/transfer` — data plane (parallel TCP streams; HTTP upload/pull for web)
- `core/webui` — embedded mobile browser page
- `app.go` / `frontend/` — Wails desktop UI

Control plane (HTTPS) and data plane (raw parallel TCP, or HTTP for browsers)
are deliberately separate so native transfers can saturate the link.

## Prerequisites

- **Go** 1.21+
- **Node.js** + npm (LTS)
- **Wails CLI**: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### Linux extra dependencies

Wails on Linux needs GTK3 and WebKit2GTK. On **Ubuntu 24.04+ / recent distros**
only WebKit2GTK **4.1** is available, which requires the `webkit2_41` build tag.
The build script below handles all of this automatically.

## Build scripts

There is one self-contained build script per platform. Each one:

- checks the current dependencies before building,
- installs only the dependencies that are missing,
- updates only the dependencies that are outdated (Go/Node by minimum version),
- builds the app and prints the build result,
- prints the binary location,
- can optionally remove exactly the dependencies it installed.

**Windows** (PowerShell):

```powershell
powershell -ExecutionPolicy Bypass -File scripts\build.ps1            # check + build
powershell -ExecutionPolicy Bypass -File scripts\build.ps1 -CheckOnly # deps only
powershell -ExecutionPolicy Bypass -File scripts\build.ps1 -Clean     # remove installed deps
```

**Linux / macOS** (bash):

```bash
bash scripts/build.sh               # check + build
bash scripts/build.sh --check-only  # deps only
bash scripts/build.sh --clean       # remove installed deps
```

The Linux script auto-detects WebKit2GTK 4.1 and builds with `-tags webkit2_41`.
It records every package it installs in `.swoop-deps.txt`, and `--clean` removes
exactly those (it never touches dependencies that were already present).

### Make shortcuts

```bash
make build       # scripts/build.sh
make check       # scripts/build.sh --check-only
make clean-deps  # scripts/build.sh --clean
make dev         # live development (hot reload)
make doctor      # wails doctor
```

## Troubleshooting

**esbuild "installed for another platform"** — this happens when the whole
project (including `frontend/node_modules`) is copied between Windows and Linux/
macOS, because esbuild ships a platform-specific native binary. The build
scripts detect a foreign `node_modules` and reinstall it automatically. To fix
it manually:

```bash
rm -rf frontend/node_modules && bash scripts/build.sh
```

> Tip: testing discovery needs **two** machines (or VMs) on the same LAN. Two
> instances on one host won't see each other — they share a device identity and
> the control port.

**Transfers fail with EOF** — pick the LAN interface explicitly at startup
(Wi‑Fi vs Ethernet vs a VM adapter).

**Phone cannot connect** — same Wi‑Fi, accept the HTTPS certificate warning,
allow port **53317** through the desktop firewall.

**Internet invite expired** — create a new invite on the host; joiner must import
within **15 minutes**.

**Internet peer disappeared** — idle timeout (**20 minutes** without chat or
transfer), app closed, or network loss. Re-pair with a fresh invite.

**Internet transfer slow or stuck on relay** — badge **R** means traffic goes
through the relay; **P2P** means a direct path was negotiated. Some NAT setups
stay on relay only.

**Diagnostics log** — the engine writes `swoop.log` under the app data folder
(same place as device identity):

| OS | Path |
|----|------|
| Windows | `%APPDATA%\Swoop\swoop.log` |
| macOS | `~/Library/Application Support/Swoop/swoop.log` |
| Linux | `~/.config/Swoop/swoop.log` |

When running from a writable build directory, a copy may also appear next to the
binary. The first log line records the exact path (`logging to …`). See also
[docs/USAGE.md](docs/USAGE.md) (Russian).
