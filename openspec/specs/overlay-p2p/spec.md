# S013 — overlay-p2p

Status: **Implemented**


## Purpose

Carry invite-paired traffic first over an overlay WebSocket relay, then upgrade
to QUIC P2P when possible, with clear UI notes on fallback.

## Requirements

### Requirement: Relay then QUIC
Invite-paired sessions SHALL start on the overlay relay and attempt a QUIC P2P
upgrade; the host SHOULD map QUIC UDP via UPnP when available.

#### Scenario: P2P fails
- **WHEN** QUIC upgrade cannot complete
- **THEN** the session SHALL continue on relay and the tile MAY show a p2pNote explaining fallback

### Requirement: Overlay keepalive and host reconnect
Overlay WebSocket SHALL use ping keepalive; the invite host SHALL reconnect the
relay if the socket drops before a joiner arrives.

#### Scenario: Host relay drop before join
- **WHEN** the host overlay socket drops prior to joiner attach
- **THEN** the host SHALL attempt to re-establish the relay connection
