# Archive: bootstrap OpenSpec capability catalog

Date: 2026-09-03

## Why

Brownfield adoption of OpenSpec for Swoop. Existing behavior from root
`AGENTS.md` was reconstructed as numbered capability specs S001–S024 so future
work can proceed propose → apply → archive without boiling the ocean each time.

## What landed

- `openspec/` with `config.yaml` (schema: spec-driven), `project.md`, `AGENTS.md`,
  `catalog.md`
- 24 capability specs under `openspec/specs/*/spec.md` (17 Implemented, 7 Planned)
- Cursor commands/skills: `.cursor/commands/opsx-*.md`, `.cursor/skills/openspec-*`
- Root `AGENTS.md` linked to the catalog

## Non-goals

- No application code changes in this bootstrap
- No global `npm install -g @fission-ai/openspec` (use `npx` / local CLI as needed)

## Validation

`openspec validate --all` → 24 passed.
