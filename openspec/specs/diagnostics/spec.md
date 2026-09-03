# S015 — diagnostics

Status: **Implemented**


## Purpose

Record enough operational detail next to the binary to diagnose bind, TLS, and
transfer failures without a special debug build.

## Requirements

### Requirement: swoop.log
The engine SHALL write `swoop.log` next to the binary and mirror to stderr,
covering control bind/serve, prepare-upload, TLS handshake errors, and transfer
lifecycle.

#### Scenario: Failed prepare-upload
- **WHEN** a prepare-upload fails
- **THEN** the failure SHALL appear in swoop.log
