# Proposal: add-versioning-rules

Status: **Done**

## Why

Agents must bump the product version on every change and know when to use
MAJOR vs MINOR vs PATCH.

## What Changes

- Added `openspec/VERSIONING.md` with bump rules and file sync list
- Wired rules into Cursor always-apply rule, OpenSpec AGENTS/config, root AGENTS.md
- Extended S016 build-branding requirements for version bump discipline
- Bumped product version **1.1.0 → 1.1.1** (PATCH: process/rules)

## Capabilities

- **Modified:** `build-branding` (S016)

## Impact

Agent workflow only; no wire protocol change.
