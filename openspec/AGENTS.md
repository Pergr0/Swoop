# OpenSpec Instructions — Swoop

Instructions for AI coding assistants using OpenSpec in this repository.

## TL;DR

- Read root `AGENTS.md`, then `openspec/project.md`, `openspec/STATUS.md`, `openspec/catalog.md`
- Behavior source of truth: `openspec/specs/<capability>/spec.md` (each has a **Status** line)
- **Every** behavior change: propose → plan (`tasks.md`) → implement → version bump → archive; update STATUS
- Version rules: `openspec/VERSIONING.md` (MAJOR / MINOR / PATCH) — always bump and sync all version files
- Capability ids: **S001–S024** (see catalog)

## Mandatory workflow

### 1. Propose (`/opsx-propose`)

Create when adding/changing behavior, architecture, or wire contracts.

Scaffold `openspec/changes/<kebab-id>/`:

| File | Role |
|------|------|
| `proposal.md` | Why / what / capabilities / impact + **Status:** line |
| `specs/<capability>/spec.md` | ADDED\|MODIFIED\|REMOVED deltas |
| `design.md` | Optional how |
| `tasks.md` | Ordered checkbox plan — this is the implementation plan |

**Change statuses** (in `proposal.md`): `Proposed` → `In progress` → `Done` | `Blocked` | `Cancelled`.

Also add/update the row in `openspec/STATUS.md` → Active changes.

Skip full proposal only for typos, pure refactors (`skip_specs: true`), docs-only with no requirement change.

### 2. Apply (`/opsx-apply`)

- Set change Status to **In progress**
- Implement `tasks.md` in order; mark `- [x]` only when verified
- Keep `core/` free of Wails/CGO
- On behavior ship: update root `AGENTS.md` §6/§7, capability `Status:`, `catalog.md`, `STATUS.md` in the same session

### 3. Version bump (before or with archive)

Follow `openspec/VERSIONING.md`:

- **MAJOR** — breaking peers/users or incompatible wire/identity
- **MINOR** — new backward-compatible feature
- **PATCH** — fix, hardening, process/rules

Sync: `frontend/src/version.ts`, `wails.json`, `frontend/package.json`,
`STATUS.md`, `project.md`, `config.yaml`.

### 4. Archive (`/opsx-archive`)

- Merge deltas into `openspec/specs/`
- Set capability Status (`Implemented` / `Planned` / `Deprecated`)
- Move change to `openspec/changes/archive/YYYY-MM-DD-<id>/`
- Clear Active changes in `STATUS.md`; refresh summary counts + product version

## Spec format (CRITICAL)

Every capability `spec.md` MUST start with:

```markdown
# S0NN — capability-name

Status: **Implemented** | **Planned** | **In progress** | **Deprecated**

## Purpose
...
```

Requirements:

```markdown
### Requirement: Short name
The system SHALL ...

#### Scenario: Short name
- **WHEN** ...
- **THEN** ...
```

Scenarios MUST use exactly `#### Scenario:`. Use SHALL/MUST.

## Directory structure

```
openspec/
├── STATUS.md               # At-a-glance dashboard (keep current)
├── catalog.md              # S001–S024 index
├── project.md
├── AGENTS.md               # this file
├── config.yaml
├── specs/<capability>/spec.md
└── changes/                # active + archive/
```

## Key files

1. `openspec/STATUS.md` — living project state
2. `openspec/catalog.md` — numbered feature index
3. Root `AGENTS.md` — implementation map + conventions
4. `docs/USAGE.md`, `docs/MOBILE-WEB.md` — user-facing docs
