# Swoop product versioning

SemVer for the **desktop app product version** (`MAJOR.MINOR.PATCH`).
This is **not** the wire `protocol.Version` (bump that only on incompatible
protocol changes; see root `AGENTS.md` / `core/protocol`).

## Always bump on change

Every shipped change that is archived (or any hotfix merged to the working tree
that alters product behavior, process rules agents must follow, UI, or
packaging) **MUST** bump the product version in the **same** change.

Do not leave `1.x.y` unchanged after a completed OpenSpec apply/archive.

## Which number to bump

| Bump | When | Examples |
|------|------|----------|
| **MAJOR** (`X.0.0`) | Breaking change for users or peers: old clients cannot interoperate, or persisted identity/config is incompatible without migration | Wire protocol bump; rename/remove control API peers rely on; force re-pair; change identity file format without backward read |
| **MINOR** (`x.Y.0`) | Backward-compatible **feature**: new capability, new UI flow, new optional API field peers may ignore | Internet pairing, version badge, new transfer mode that old peers still refuse cleanly |
| **PATCH** (`x.y.Z`) | Backward-compatible **fix**, hardening, docs/process/rules, refactors with no user-facing feature | Nil-panic fix, lazy FD open, OpenSpec rules, copy/i18n tweaks, build script fixes |

### Decision tips

- If unsure between MINOR and PATCH: does the user gain a new capability or
  visible workflow? → **MINOR**. Otherwise → **PATCH**.
- If old Swoop on the LAN would misbehave or fail in a new way against this
  build (not just miss a feature) → **MAJOR** (and bump `protocol.Version` if
  the wire changed).
- Roadmap-only doc updates with no code: still **PATCH** if agent/process
  rules or STATUS/catalog semantics change; skip bump only for typo-only
  edits that do not change meaning (`skip_specs` docs).

## Files that MUST stay in sync

Update **all** of these to the same `MAJOR.MINOR.PATCH` string:

1. `frontend/src/version.ts` → `APP_VERSION` (header badge)
2. `wails.json` → `info.productVersion`
3. `frontend/package.json` → `version`
4. `openspec/STATUS.md` → **Product version**
5. `openspec/project.md` → App version section
6. `openspec/config.yaml` → context line mentioning product version

Optional: mention the new version in the OpenSpec change `proposal.md`.

## Git tags

Prefer annotated tags `vMAJOR.MINOR.PATCH` on release builds. Day-to-day
agent work bumps the files above even before a tag is cut.

## Checklist (every apply/archive)

- [ ] Chose MAJOR / MINOR / PATCH per the table
- [ ] All six sources list the same version
- [ ] Header will show `vX.Y.Z` after rebuild
- [ ] `openspec/STATUS.md` **Last status update** refreshed
