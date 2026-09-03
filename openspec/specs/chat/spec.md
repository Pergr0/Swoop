# S009 — chat

Status: **Implemented**


## Purpose

Exchange short per-peer text messages over the pinned control channel with
delivery/read receipts and session-scoped persistence.

## Requirements

### Requirement: Inert validated messages
Chat messages SHALL be validated (UTF-8, <= 8 KiB), rate-limited per peer, stored
as JSON Lines in a temp file deleted on shutdown, and rendered as escaped text
only (never executed).

#### Scenario: Message with HTML
- **WHEN** a peer sends a message containing markup
- **THEN** the UI SHALL display it as plain escaped text

### Requirement: Delivery and read receipts
Outgoing messages SHALL show delivered after HTTP 200 and read after the peer
posts a read receipt for the shared message timestamp.

#### Scenario: Peer opens chat
- **WHEN** the receiver has the chat open and receives a message
- **THEN** the sender's UI SHALL eventually show a read tick for that message
