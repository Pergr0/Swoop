# S007 — transfer

Status: **Implemented**


## Purpose

Move file bytes between native peers quickly and safely: control-plane accept
handshake, then parallel TCP streams with live progress.

## Requirements

### Requirement: Accept/decline handshake
A send SHALL use a control-plane prepare-upload handshake that blocks until the
receiver accepts or declines.

#### Scenario: Receiver declines
- **WHEN** the receiver declines an offer
- **THEN** the sender SHALL abort without opening the data plane for that session

### Requirement: Parallel TCP data plane
Accepted transfers SHALL use N parallel TCP streams (default 4) with per-file
range splitting. When a batch mixes small files with large ones (>= chunk size),
at least one stream SHALL be reserved for large-file ranges.

#### Scenario: Mixed small and large files
- **WHEN** a batch contains tiny files and at least one large file
- **THEN** large-file ranges SHALL still make progress (not starved by small files)

### Requirement: Lazy file descriptors
Senders and receivers SHALL open source/destination file handles lazily per
worker (not all files at once), and on Unix the process SHALL raise the FD soft
limit at startup so large folder sends do not crash the UI.

#### Scenario: Large folder send on macOS
- **WHEN** the user sends a folder with many files
- **THEN** the app SHALL NOT exhaust the default ~256 FD soft limit by opening all files up front

### Requirement: One session each way
The system SHALL allow at most one outgoing and one incoming transfer session at
a time.

#### Scenario: Second send while busy
- **WHEN** an outgoing transfer is active
- **THEN** starting another outgoing send SHALL be rejected or queued per existing UI rules
