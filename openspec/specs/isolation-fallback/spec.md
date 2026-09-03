# S021 — isolation-fallback

Status: **Planned**

## Purpose

Provide QR / manual-IP connection when client isolation blocks multicast
discovery between devices on the same Wi-Fi.

## Requirements

### Requirement: Manual peer add
The user SHALL be able to add a peer by scanning a QR code or entering host
address and port when discovery finds nobody.

#### Scenario: Isolated AP
- **WHEN** multicast discovery yields no peers but the user scans a valid peer QR
- **THEN** the peer SHALL appear and be usable for transfer
