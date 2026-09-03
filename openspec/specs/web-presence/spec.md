# S011 — web-presence

Status: **Implemented**


## Purpose

Show active browser clients in the desktop device grid and authorize their
pull/download APIs with a short-lived HMAC token.

## Requirements

### Requirement: Presence heartbeats
Browser clients SHALL appear in the grid via POST /api/v1/presence heartbeats,
with platform web and a parsed User-Agent browser label.

#### Scenario: Phone opens upload page
- **WHEN** a browser posts presence heartbeats to a desktop
- **THEN** a web tile SHALL appear and remain while heartbeats continue

### Requirement: Web token binding
Each heartbeat SHALL return an HMAC token bound to clientId and source IP,
required on pull-offer/respond/download via X-Swoop-Web-Token.

#### Scenario: Missing token
- **WHEN** a pull-offer request omits a valid web token
- **THEN** the desktop SHALL reject the request
