# S003 — netif

Status: **Implemented**


## Purpose

Let the user choose which network interface Swoop binds and advertises so
transfers succeed on multi-adapter hosts (VPN, VM NAT, etc.).

## Requirements

### Requirement: Startup interface picker
The system SHALL present a one-time startup picker listing up, non-loopback IPv4
interfaces (name, addresses, kind icon, best-effort link speed) plus an Auto
option.

#### Scenario: User selects ethernet
- **WHEN** the user selects a specific interface and starts the engine
- **THEN** discovery and advertised IP SHALL use that interface

#### Scenario: Auto mode
- **WHEN** the user chooses Auto
- **THEN** discovery SHALL join/broadcast on all eligible interfaces (legacy behavior)

### Requirement: Not hot-swappable
The system SHALL ignore attempts to restart the engine with a different
interface in the same process after a successful start.

#### Scenario: Re-start ignored
- **WHEN** StartEngine is called again after a successful start
- **THEN** the running bind/advertise choice SHALL remain unchanged
