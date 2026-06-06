# Swoop

Zero-config file transfer for the local network. Devices running Swoop discover
each other automatically and exchange files directly (no cloud, no setup),
across Windows, macOS and Linux — with mobile planned.

## Architecture (short)

Protocol-first: a platform-agnostic Go core (`core/`) implements the protocol,
and the UI is just one client of it.

- `core/protocol` — wire types and constants (one source of truth)
- `core/identity` — device id + self-signed TLS cert + fingerprint (TOFU trust)
- `core/discovery` — peer discovery via UDP multicast on all interfaces
- `core/transport` — control plane (HTTPS `/api/v1/...`: info, handshake)
- `core/transfer` — data plane (parallel TCP streams, built for throughput)
- `app.go` / `frontend/` — Wails desktop UI

Control plane (HTTPS) and data plane (raw parallel TCP) are deliberately
separate so file bytes can saturate the link without HTTP overhead.

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

**esbuild "installed for another platform"** - this happens when the whole
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
