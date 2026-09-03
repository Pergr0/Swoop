# S016 — build-branding

Status: **Implemented**


## Purpose

Produce reproducible desktop builds with the Swoop mark and a consistent product
version across platforms.

## Requirements

### Requirement: Icon from appicon.png
Every platform build SHALL refresh icons from `build/appicon.png` (no stale Wails
default mark).

#### Scenario: Clean Windows build
- **WHEN** scripts/build.ps1 runs a full build
- **THEN** the packaged icon SHALL derive from build/appicon.png

### Requirement: Product version metadata
The release SHALL carry the current cataloged product version (see
`openspec/STATUS.md` and `openspec/VERSIONING.md`) in Wails product metadata and
the UI badge (S006). Every archived behavior or process change SHALL bump
MAJOR, MINOR, or PATCH per VERSIONING.md and keep all version sources identical.

#### Scenario: Version alignment
- **WHEN** a release or archived change is cut
- **THEN** wails.json productVersion and frontend APP_VERSION SHALL match
- **AND** openspec/STATUS.md Product version SHALL show the same value

#### Scenario: Patch bump for process rules
- **WHEN** agent/process rules change without a new user-facing capability
- **THEN** the product PATCH version SHALL increase
