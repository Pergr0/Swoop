# S017 — paths-downloads

Status: **Implemented**


## Purpose

Land received files in the user's real OS Downloads folder and allow revealing
that folder in the native file manager.

## Requirements

### Requirement: Platform Downloads resolution
Received files SHALL be written under the platform Downloads directory
(Windows known folder, Linux XDG user-dirs, macOS ~/Downloads), preserving
relative paths from the offer.

#### Scenario: Localized Linux Downloads
- **WHEN** XDG user-dirs names Downloads in the user locale
- **THEN** Swoop SHALL resolve that path rather than assuming ~/Downloads

### Requirement: Reveal Downloads
The UI SHALL be able to open the Downloads folder via the OS file manager.

#### Scenario: User clicks open folder
- **WHEN** the user activates RevealDownloads after a receive
- **THEN** the OS file manager SHALL open at the Downloads location
