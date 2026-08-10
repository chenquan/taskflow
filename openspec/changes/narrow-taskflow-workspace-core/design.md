## Context

Taskflow currently persists a user configuration, a repository inventory snapshot, execution state, and validation reports. It also stores task-specific Codex/Claude executable policy even though `open` only needs a deterministic mapping from an already prepared workspace to a child process. Status derives publication and dependency-readiness booleans that its Git observations cannot fully establish. The CLI is young enough to take a deliberate breaking change, and the user selected rejection rather than automatic migration for existing task workspaces.

## Goals / Non-Goals

**Goals:**

- Preserve safe multi-repository worktree creation, recovery, validation, and one-command Codex/Claude launch.
- Reduce persisted sources of truth to `taskflow.yaml`, `.taskflow/state.json`, and validation reports.
- Make launch and status semantics directly testable from runtime facts.
- Make the breaking boundary explicit and preserve every legacy file on rejection.

**Non-Goals:**

- Migrating existing task configurations or state.
- Adding generic tool providers, session tracking, task completion, Git publication, cleanup, requirements, ownership, or contract management.
- Changing the worktree creation and source-lock safety model.

## Decisions

### Use repository order as the primary selection

The first configured repository is always the cwd for `open`; all later repositories and the task root are additional directories. `--primary` is removed because the current domain does not serialize that selection and append-only repository growth already keeps the first entry stable. Persisting a new primary field was rejected because it adds policy without improving the common path.

### Keep two fixed launch adapters

`open` defaults to Codex and accepts `--tool codex|claude`. Executables are resolved as `codex` or `claude` from `PATH`; task configuration cannot override them. Extra arguments after `--` are forwarded unchanged except `--worktree` and `--worktree=...`, which are rejected to prevent nested worktree creation. Claude always receives `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1`. This keeps the useful launch path without becoming a tool plugin or policy system.

### Gate launch on reconciled workspace facts

Before spawning a tool, `open` loads schema-v2 state, requires phase `started`, and validates that every configured target is a worktree from the configured Git common directory on the expected branch. Dirty worktrees are allowed. Failure returns a structured diagnostic before child execution.

### Remove inventory rather than strengthen it

New initialization writes only YAML and state. Repository append snapshots and rolls back only those two files. Existing `inventory.json` files are never read, updated, or deleted. Live Git inspection remains authoritative for repository identity and conflict checks.

### Report facts rather than readiness conclusions

Status retains raw branch, HEAD, dirty, upstream, ahead, behind, worktree errors, phase, and the historical validation report. It removes `pushed`, `dependencyReady`, and per-repository `lastValidationOK`. `validationConfigStale` only compares the report configuration digest to the current normalized configuration and does not claim that code is complete or currently validated.

### Make the breaking boundary explicit

State and validation report schemas advance to version 2. A YAML `development` key receives `LEGACY_CONFIGURATION_UNSUPPORTED`; schema-v1 state/report data is rejected or ignored as incompatible without mutation. Users reinitialize in an empty task directory or another tasks root. No automatic cleanup is performed.

## Risks / Trade-offs

- [Existing workspaces stop loading] → Return precise migration guidance and preserve all files and worktrees.
- [Fixed executable names reduce customization] → Rely on normal `PATH` selection and keep tool arguments fully forwardable.
- [Historical validation can be mistaken for current validation] → Name config staleness narrowly and expose current Git facts beside the report.
- [Removing inventory weakens snapshot auditing] → Continue canonical live Git common-directory and branch verification before mutations and launch.
- [Breaking JSON affects consumers] → Advance report/state schemas, update the bundled skill, docs, and E2E fixtures in the same release.

## Migration Plan

1. Fast-forward to the current `origin/main` baseline and land this as one breaking release.
2. New tasks use the narrowed schema and state version 2.
3. Legacy tasks fail read-only with explicit reinitialization guidance; users create a new empty task workspace and retain the old directory until they choose to remove it manually.
4. Rollback is a code release rollback; legacy task files were never rewritten, so the prior binary can still read them.

## Open Questions

None.
