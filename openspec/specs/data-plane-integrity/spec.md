# S018 — data-plane-integrity

Status: **Planned**

## Purpose

Harden the data channel with integrity, resume, and encryption beyond today's
token-authenticated plaintext LAN streams.

## Requirements

### Requirement: Per-file integrity
The system SHALL verify each transferred file with a sha256 digest agreed in the
offer/handshake.

#### Scenario: Corrupt payload
- **WHEN** received bytes do not match the expected digest
- **THEN** the transfer SHALL fail closed for that file

### Requirement: Resumable transfers
The system SHALL support resuming interrupted transfers without re-sending
completed ranges.

#### Scenario: Mid-transfer disconnect
- **WHEN** a transfer is interrupted after partial progress
- **THEN** a retry SHALL continue from the last confirmed offset

### Requirement: Data-plane AEAD
The data channel SHALL use AEAD encryption so file bytes are confidential on the
wire even on hostile LANs.

#### Scenario: Passive observer
- **WHEN** an observer captures data-plane packets
- **THEN** file contents SHALL not be recoverable without session keys
