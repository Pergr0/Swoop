# Swoop — OpenSpec Project Context

## Purpose

Zero-config file transfer on the local network (and optionally across the
internet via invite + relay/P2P). Devices discover each other and exchange
files directly — no cloud storage, no manual SMB/FTP setup. Desktop:
Windows / macOS / Linux. Mobile today: HTTPS web UI in the phone browser.
Native mobile apps remain roadmap (S024).

## Design principles

1. **Zero-config UX** — tiles, drag, Accept; no IP entry for LAN peers.
2. **Speed** — dedicated data plane (parallel TCP) separate from HTTPS control.
3. **Cross-platform core** — Go `core/` is stdlib + x/net only, no CGO, no Wails.
4. **Protocol-first** — UI is a client of the core engine.
5. **Security without friction** — TLS + fingerprint pinning (TOFU UI is S019).

## Stack

| Layer | Choice |
|-------|--------|
| Core | Go (`swoop` module), `core/` |
| Desktop | Wails v2 |
| Frontend | Svelte + Vite + TypeScript |
| Rendezvous | `cmd/` server + `make build-server` deploy bundle |
| Specs | `openspec/` (this tree) |

## App version

**1.1.1** — shown in the header (`frontend/src/version.ts`), also
`wails.json` `info.productVersion`. Bump rules: [`VERSIONING.md`](./VERSIONING.md).
Last git tag before this line of work: `v1.0.0`.

## Spec index

See [`catalog.md`](./catalog.md) for numbered capabilities **S001–S024**.

## Workflow

**Mandatory for behavior work:**

1. `/opsx-propose` — proposal + delta specs + `tasks.md` plan; Status on proposal
2. Update `STATUS.md` Active changes
3. `/opsx-apply` — implement against the plan; tick tasks; keep Status current
4. `/opsx-archive` — merge specs, set capability Status, refresh STATUS/catalog

Always read `STATUS.md` + `catalog.md` + touched capability specs + root
`AGENTS.md` before proposing.

