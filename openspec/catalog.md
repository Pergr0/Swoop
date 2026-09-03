# Swoop Capability Catalog

Numbered index of OpenSpec capabilities. **Living dashboard:** [`STATUS.md`](./STATUS.md).

Status column must stay aligned with each `specs/<capability>/spec.md` `Status:`
line and with STATUS.md tables.

| ID | Capability | Status | Summary |
|----|------------|--------|---------|
| S001 | `identity` | Implemented | Stable device id, self-signed TLS cert, fingerprint persistence |
| S002 | `discovery` | Implemented | UDP multicast peer discovery, ordered grid, advertise IP |
| S003 | `netif` | Implemented | Startup network interface picker (Auto / specific iface) |
| S004 | `control-plane` | Implemented | HTTPS `/api/v1/*` control plane, timeouts, panic recovery |
| S005 | `engine-desktop` | Implemented | Engine orchestration + Wails bindings / events |
| S006 | `desktop-ui` | Implemented | Device grid, staging UI, progress, QR, i18n, version badge |
| S007 | `transfer` | Implemented | Accept/decline handshake + parallel TCP data plane |
| S008 | `staging` | Implemented | Directory scan, checklist, offer root summaries, file caps |
| S009 | `chat` | Implemented | Per-peer text chat, receipts, inert rendering |
| S010 | `mobile-web` | Implemented | Embedded browser UI; phone↔desktop upload/pull |
| S011 | `web-presence` | Implemented | Browser heartbeats in grid + HMAC web token |
| S012 | `internet-pairing` | Implemented | Signed invite, rendezvous room, idle goodbye |
| S013 | `overlay-p2p` | Implemented | Overlay WebSocket relay, QUIC upgrade, UPnP, keepalive |
| S014 | `hardening` | Implemented | Offer validation, size caps, session limits, fail-closed pin |
| S015 | `diagnostics` | Implemented | `swoop.log`, stderr mirror, transfer/control lifecycle logs |
| S016 | `build-branding` | Implemented | Cross-platform build scripts, app icon, product version |
| S017 | `paths-downloads` | Implemented | OS Downloads folder resolution + reveal |
| S018 | `data-plane-integrity` | Planned | Per-file sha256, resumable transfers, data-plane AEAD |
| S019 | `trust-ui` | Planned | Explicit TOFU confirmation (PIN/QR) for new peers |
| S020 | `mdns` | Planned | mDNS/DNS-SD discovery layered on multicast |
| S021 | `isolation-fallback` | Planned | QR / manual IP when multicast is blocked |
| S022 | `windows-firewall` | Planned | First-run firewall prompt/rule on Windows |
| S023 | `mobile-polish` | Planned | Resumable Range pull, WebSocket progress on phone |
| S024 | `native-mobile` | Planned | Native iOS/Android clients (gomobile or protocol ports) |

## How to use

- Changing behavior for an Implemented row → OpenSpec change with
  **Modified Capabilities** pointing at that folder; keep Status on the change
  and on the capability spec accurate.
- Starting a Planned row → change that **Adds**/promotes the capability; flip
  Status to Implemented on archive.
- Keep this table, `STATUS.md`, and each `spec.md` Status line in sync.
