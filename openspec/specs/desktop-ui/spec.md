# S006 — desktop-ui

Status: **Implemented**


## Purpose

Provide a readable desktop UI for discovery, staging, sending/receiving, chat,
QR mobile pairing, and clear product versioning.

## Requirements

### Requirement: Device grid and self card
The UI SHALL show a device grid with this-device card, platform icons, discovery
status, and empty state when no peers are present.

#### Scenario: App started with no peers
- **WHEN** the engine is running and no peers are known
- **THEN** the UI SHALL show an empty/discovery state rather than stale tiles

### Requirement: Transfer and offer UX
The UI SHALL provide drop-zone staging, Send, progress (including indeterminate
while waiting for accept), cancel, incoming-offer modal with preview, and reveal
Downloads.

#### Scenario: Incoming offer
- **WHEN** a peer offers files
- **THEN** the receiver UI SHALL show an accept/decline modal with file preview

### Requirement: Header product version
The UI SHALL show the product version immediately to the right of the Swoop
title using the same muted small typography as the discovery status line.

#### Scenario: Version visible
- **WHEN** the main window is shown
- **THEN** the brand row SHALL include a version marker such as v1.1.0
