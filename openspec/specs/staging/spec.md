# S008 — staging

Status: **Implemented**


## Purpose

Scan local files and directories into a sender checklist and summarize roots for
the receiver offer modal.

## Requirements

### Requirement: Recursive directory staging
The system SHALL scan nested directories and multiple root folders into a staging
tree with per-file/dir selection.

#### Scenario: Nested folder drop
- **WHEN** the user stages a directory tree
- **THEN** relative paths SHALL be preserved for the eventual Downloads layout

### Requirement: File count cap
Staging SHALL reject batches that exceed the protocol maximum transfer file
count.

#### Scenario: Too many files
- **WHEN** a scan would exceed MaxTransferFiles
- **THEN** staging SHALL fail with a clear user-visible error
