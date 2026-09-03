# S002 — discovery

Status: **Implemented**


## Purpose

Let devices on the same LAN find each other without manual IP entry via UDP
multicast announcements, with a stable on-screen peer order.

## Requirements

### Requirement: Multicast announce and listen
The system SHALL announce itself and listen for peers using UDP multicast on
every up, multicast-capable interface, or on a single interface when one is
selected (see S003).

#### Scenario: Peer appears on LAN
- **WHEN** another Swoop device announces on the same LAN
- **THEN** it SHALL appear in the local peer set with name and address:port

### Requirement: Advertised address preferred
The system SHALL address a peer using the IP that peer advertises, falling back
to the packet source address only when none is advertised.

#### Scenario: Multi-homed peer
- **WHEN** a peer advertises a specific interface IP
- **THEN** transfers and control calls SHALL target that advertised IP

### Requirement: Stable first-seen order
The system SHALL keep a session-scoped first-seen order so the device grid does
not reshuffle on each poll; peers unseen for approximately 12 seconds SHALL be
dropped and, if they return, reappear at the end.

#### Scenario: Peer briefly flaps
- **WHEN** a peer is silent longer than the expiry window
- **THEN** it SHALL be removed from the grid
- **AND WHEN** it announces again
- **THEN** it SHALL be appended after currently visible peers
