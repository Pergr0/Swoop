# S023 — mobile-polish

Status: **Planned**

## Purpose

Improve the browser client with resumable Range downloads and live WebSocket
progress.

## Requirements

### Requirement: Range pull resume
Phone downloads SHALL support HTTP Range resume for interrupted pulls.

#### Scenario: Download interrupted
- **WHEN** a phone pull disconnects mid-file
- **THEN** a retry SHALL resume with Range rather than restarting from zero

### Requirement: Live progress on phone
The phone UI SHALL receive live transfer progress over a WebSocket (or equivalent
push channel).

#### Scenario: Large download
- **WHEN** a multi-hundred-MB archive is downloading to the phone
- **THEN** the phone UI SHALL update progress without relying only on coarse poll intervals
