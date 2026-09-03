# S005 — engine-desktop

Status: **Implemented**


## Purpose

Wire identity, discovery, transport, and transfer into a single engine and
expose it to the desktop UI through Wails bindings and events.

## Requirements

### Requirement: Engine start after successful bind
The engine SHALL mark itself started only after control-plane bind succeeds, and
Close SHALL cancel networking cleanly.

#### Scenario: Bind failure
- **WHEN** the control port cannot be bound
- **THEN** the engine SHALL NOT report a successful start to the UI

### Requirement: Live peer updates
The desktop adapter SHALL expose SelfInfo and Peers and emit a `peers:changed`
event when the peer set changes, with panic-contained callbacks.

#### Scenario: Peer join
- **WHEN** discovery adds a peer
- **THEN** the frontend SHALL receive an updated peer list via peers:changed
