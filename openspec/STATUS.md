# Swoop — project status

Living dashboard. Update on every archived or in-progress OpenSpec change.
Capability details: [`catalog.md`](./catalog.md) · contracts: [`specs/`](./specs/).

**Product version:** 1.1.1  
**Wire protocol:** 1  
**Last status update:** 2026-09-03  
**Versioning rules:** [`VERSIONING.md`](./VERSIONING.md)

## Summary

| Bucket | Count | Notes |
|--------|------:|-------|
| Implemented | 17 | S001–S017 |
| In progress | 0 | no active `openspec/changes/` (non-archive) |
| Planned | 7 | S018–S024 |
| Blocked | 0 | — |

## Active changes

_None._ Start work with `/opsx-propose <kebab-id>`.

## Implemented (S001–S017)

| ID | Capability | Notes |
|----|------------|-------|
| S001 | identity | Stable TLS identity + fingerprint |
| S002 | discovery | LAN multicast, ordered grid |
| S003 | netif | Startup interface picker |
| S004 | control-plane | HTTPS `/api/v1` |
| S005 | engine-desktop | Engine + Wails |
| S006 | desktop-ui | Grid, transfer UI, version badge |
| S007 | transfer | Accept + parallel TCP; lazy FDs |
| S008 | staging | Dir scan + caps |
| S009 | chat | Text + receipts |
| S010 | mobile-web | Browser up/download |
| S011 | web-presence | Heartbeats + HMAC token |
| S012 | internet-pairing | Invite + rendezvous idle |
| S013 | overlay-p2p | Relay + QUIC upgrade |
| S014 | hardening | Caps, pin fail-closed |
| S015 | diagnostics | swoop.log |
| S016 | build-branding | Scripts + icon + version |
| S017 | paths-downloads | OS Downloads + reveal |

## Planned (S018–S024)

| ID | Capability | Notes |
|----|------------|-------|
| S018 | data-plane-integrity | sha256, resume, AEAD |
| S019 | trust-ui | Explicit TOFU PIN/QR |
| S020 | mdns | DNS-SD alongside multicast |
| S021 | isolation-fallback | QR / manual IP |
| S022 | windows-firewall | First-run rule |
| S023 | mobile-polish | Range pull, WS progress |
| S024 | native-mobile | iOS/Android apps |

## Recent archive

| Date | Change | Result |
|------|--------|--------|
| 2026-09-03 | `bootstrap-capability-catalog` | OpenSpec adopted; S001–S024 written |
| 2026-09-03 | _(hotfix, no change folder)_ | LAN send typed-nil overlay panic fixed in `engine.go` |
| 2026-09-03 | `add-versioning-rules` (PATCH → 1.1.1) | VERSIONING.md + always-bump policy |

## How agents must update this file

1. Opening a change → add a row under **Active changes**, set status In progress.
2. Completing tasks → keep the active row accurate (`Blocked` if stuck).
3. Archiving → move capability rows between tables, bump **Last status update**, clear Active.
4. Never leave STATUS stale relative to `catalog.md` or root `AGENTS.md` §6/§7.
