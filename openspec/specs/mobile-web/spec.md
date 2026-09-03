# S010 — mobile-web

Status: **Implemented**


## Purpose

Allow phones/tablets to exchange files with a desktop host through the embedded
HTTPS web UI without installing a native app.

## Requirements

### Requirement: Phone to desktop upload
A paired browser SHALL upload via prepare-upload plus multipart
POST /api/v1/upload/{session}.

#### Scenario: Phone sends a photo
- **WHEN** the browser user accepts the desktop offer flow for upload
- **THEN** the file SHALL land in the desktop Downloads tree

### Requirement: Desktop to phone pull
Desktop send to a web peer SHALL create a pull offer; the phone SHALL poll,
accept, and download via single-file or archive endpoints.

#### Scenario: Folder to phone
- **WHEN** the desktop sends multiple files/folders to a web peer and the phone accepts
- **THEN** the phone SHALL receive a zip archive that is deleted on the desktop after transfer
