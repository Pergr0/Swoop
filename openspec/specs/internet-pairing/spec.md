# S012 — internet-pairing

Status: **Implemented**


## Purpose

Pair two Swoop devices across networks using a short-lived signed invite and a
rendezvous server, without disturbing LAN discovery for local peers.

## Requirements

### Requirement: Signed invite expiry
A signed invite blob SHALL expire in 15 minutes if nobody joins.

#### Scenario: Stale invite
- **WHEN** a joiner presents an invite older than 15 minutes
- **THEN** pairing SHALL fail

### Requirement: Idle teardown
Invite-paired peers SHALL use a 20-minute idle timer after the last chat message
or finished/canceled transfer; an active transfer (including waiting for accept)
SHALL pause the timer. On expiry both sides SHALL send goodbye and remove the tile.

#### Scenario: Long transfer
- **WHEN** an invite-paired transfer runs longer than 20 minutes
- **THEN** the idle timer SHALL NOT tear down the peer while the transfer is active

### Requirement: LAN peers unchanged
LAN-discovered peers SHALL remain visible without consuming a relay slot.

#### Scenario: Same-LAN peer
- **WHEN** two devices are on the same LAN
- **THEN** they SHALL continue to use LAN discovery/paths even if invite features exist
