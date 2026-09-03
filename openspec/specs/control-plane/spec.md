# S004 — control-plane

Status: **Implemented**


## Purpose

Expose a TLS HTTPS API for small control messages (info, handshake, chat,
presence, invites) separate from the high-throughput data plane.

## Requirements

### Requirement: HTTPS info endpoint
The system SHALL serve `GET /api/v1/info` over TLS using the device certificate.

#### Scenario: Peer probes info
- **WHEN** a client calls GET /api/v1/info on the control port
- **THEN** the response SHALL include device identity fields needed for pairing

### Requirement: Server timeouts and panic recovery
The control-plane HTTP server SHALL use read/write/idle timeouts, and handlers
SHALL recover from panics with HTTP 500 instead of closing the connection bare.

#### Scenario: Handler panic
- **WHEN** a control handler panics
- **THEN** the peer SHALL receive an HTTP 500 rather than a silent EOF
