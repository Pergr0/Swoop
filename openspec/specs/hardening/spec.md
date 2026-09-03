# S014 — hardening

Status: **Implemented**


## Purpose

Bound resource use and reject unsafe offers so a malicious or buggy peer cannot
easily crash or overwhelm a Swoop node.

## Requirements

### Requirement: Offer validation
Incoming offers SHALL be validated for file count, per-file size, and total size
caps; prepare-upload JSON bodies SHALL be size-capped; TCP chunk ranges SHALL be
checked against FileMeta.Size.

#### Scenario: Oversized offer
- **WHEN** an offer exceeds configured caps
- **THEN** the receiver SHALL reject it before accepting data

### Requirement: Fail-closed fingerprint pinning
Native desktop sends and discovery handling SHALL reject empty peer fingerprints.

#### Scenario: Empty fingerprint peer
- **WHEN** a native send targets a peer with an empty fingerprint
- **THEN** the send SHALL be rejected
