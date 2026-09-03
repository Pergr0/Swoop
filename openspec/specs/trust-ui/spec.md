# S019 — trust-ui

Status: **Planned**

## Purpose

Add an explicit trust-on-first-use confirmation (PIN/QR) before pinning a new
peer fingerprint in the UI.

## Requirements

### Requirement: First-seen confirmation
When a new peer fingerprint is observed, the UI SHALL require user confirmation
(PIN and/or QR compare) before treating the peer as trusted for future pins.

#### Scenario: New peer appears
- **WHEN** a previously unseen fingerprint is offered
- **THEN** the user SHALL be prompted to confirm before silent trust is stored
