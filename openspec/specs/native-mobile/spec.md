# S024 — native-mobile

Status: **Planned**

## Purpose

Ship native iOS/Android clients that speak the same Swoop protocol as the
desktop core (gomobile bind or independent implementations).

## Requirements

### Requirement: Protocol-compatible mobile apps
Native mobile apps SHALL interoperate with desktop Swoop for discovery (or
pairing) and file transfer without requiring the embedded browser UI.

#### Scenario: Phone to laptop natively
- **WHEN** a native mobile client and a desktop client are paired or on the same LAN
- **THEN** they SHALL complete an accept/decline file transfer end-to-end
