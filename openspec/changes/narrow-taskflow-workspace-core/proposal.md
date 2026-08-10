## Why

Taskflow's durable value is safe multi-repository worktree coordination and one-command AI tool launch, but its task model still carries unused repository inventory, configurable tool policy, and readiness claims that exceed what the runtime can prove. Tightening the contract now keeps the CLI small and makes every persisted field and reported status operationally meaningful.

## What Changes

- **BREAKING** Remove task-level `development` configuration and resolve the built-in Codex and Claude executables from `PATH`.
- Keep `open` as the fast path from a prepared multi-repository workspace to Codex or Claude, defaulting to Codex and using the first repository as the working directory.
- Require `open` to verify the started phase and every configured worktree before launching.
- **BREAKING** Remove `--primary`; repository order defines the stable primary repository.
- Stop creating or consuming `.taskflow/inventory.json`.
- **BREAKING** simplify status JSON to observable Git facts, remove `pushed`, `dependencyReady`, and per-repository `lastValidationOK`, and rename `validationStale` to `validationConfigStale`.
- **BREAKING** support only the current configuration and state schemas; no migration or compatibility path is provided.
- Narrow Taskflow documentation and its bundled skill to worktree preparation, tool launch, status, and configured validation; explicitly exclude requirements, delivery ownership, sessions, Git publication, and cleanup.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `development-tool-sessions`: Narrow `open` to two built-in launch adapters with readiness enforcement and no task-level tool policy.
- `task-workspace-initialization`: Remove primary selection, development configuration, and inventory creation; define the first repository as primary.
- `repository-append`: Remove inventory from the append transaction while preserving append-only digest advancement and rollback.
- `aggregate-status-validation`: Report raw status and historical validation facts without unsupported readiness or publication conclusions.
- `task-configuration-validation`: Enforce the current configuration and state schemas.
- `taskflow-multirepo-skill`: Align agent guidance with the narrowed lifecycle and remove unsupported ownership and inventory instructions.

## Impact

The Cobra command surface, task domain/configuration, tool adapter, state/report schemas, initialization and repository-append persistence, status JSON, README, bundled skill, OpenSpec requirements, unit tests, and Git-backed E2E tests are affected. Existing Taskflow workspaces must be reinitialized for the current release.
