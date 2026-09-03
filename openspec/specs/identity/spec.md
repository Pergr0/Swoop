# S001 — identity

Status: **Implemented**


## Purpose

Provide each Swoop install with a stable cryptographic device identity and TLS
certificate so peers can address and pin one another across sessions.

## Requirements

### Requirement: Persistent device identity
The system SHALL generate an ECDSA key pair and self-signed TLS certificate on
first run and persist them under the OS config directory so the device id and
fingerprint remain stable across restarts.

#### Scenario: First launch creates identity
- **WHEN** Swoop starts with no stored identity
- **THEN** it SHALL create and persist key material under the platform config dir
- **AND** subsequent launches SHALL reuse the same device id and fingerprint

### Requirement: Fingerprint for TOFU pinning
The system SHALL expose a sha256 fingerprint of the device certificate for peer
pinning on the control channel.

#### Scenario: Fingerprint available to UI and peers
- **WHEN** a peer or the local UI requests device info
- **THEN** the fingerprint SHALL be present and non-empty for native desktop peers
