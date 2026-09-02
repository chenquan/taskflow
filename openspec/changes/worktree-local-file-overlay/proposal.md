## Why

`git worktree add` creates a checkout from a commit, but it intentionally does not copy local files that are outside Git's tracked state. This makes a newly prepared Taskflow workspace lose useful repository-local configuration such as development environment files, while blindly copying every local file could leak secrets or bring in IDE and build artifacts.

Taskflow needs an explicit, reviewable local-file overlay that is safe to materialize, does not overwrite an existing worktree, and can recover from an interrupted copy.

## What Changes

- Add an optional per-repository local overlay declaration to `taskflow.yaml`, containing explicit source-relative files or directories to copy.
- Add a repeatable bootstrap option for declaring overlay paths while creating a new task; existing task configuration remains user-owned and is edited directly.
- Extend create dry-run and execute output with overlay discovery, copy, skip, and conflict actions.
- Validate overlay paths and file types before any taskflow or Git mutation; reject path escapes, `.git` content, unsupported special files, and collisions with the selected base checkout.
- Materialize the overlay only for a newly created Taskflow worktree, using no-overwrite and atomic per-file writes; a matching manually managed worktree is never modified implicitly.
- Persist a narrow overlay materialization manifest so an interrupted copy can be resumed without refreshing a completed worktree or overwriting user changes.
- Keep copied overlay files subject to the existing dirty-worktree deletion safety gate; delete will not silently remove or overwrite them.
- Update the CLI contract, configuration validation, reconciliation/recovery behavior, bundled skill, README, and end-to-end tests.

## Capabilities

### New Capabilities

- `worktree-local-file-overlay`: Explicit local-file selection, snapshot validation, safe materialization, and recovery for newly created worktrees.

### Modified Capabilities

- `task-workspace-initialization`: Bootstrap and persist local overlay declarations as part of a new task configuration.
- `worktree-start`: Include overlay planning and materialization in create dry-run and execute semantics.
- `worktree-reconciliation`: Distinguish complete, pending, and conflicting overlay materialization while preserving safe reuse behavior.
- `task-configuration-validation`: Validate overlay declarations as source-relative paths with strict YAML decoding.
- `environment-preflight`: Validate selected source files, base-tree collisions, and target safety before mutation.
- `resumable-action-execution`: Define recovery for an interrupted overlay materialization without treating partial content as complete.
- `cli-output-contract`: Report overlay actions and stable overlay-related diagnostics in text and JSON.
- `task-deletion`: Preserve the dirty-worktree gate for worktrees containing copied overlay files.
- `taskflow-multirepo-skill`: Guide agents through declaring, previewing, and executing local overlays.
- `e2e-command-flow`: Cover overlay creation, conflict prevention, retry, and deletion safety.

## Impact

- Affected code: `cmd/root.go`, `internal/app`, `internal/config`, `internal/domain`, `internal/git`, `internal/fsx`, `internal/ownership`, and plan/reporting packages.
- Affected persisted artifacts: `taskflow.yaml` gains an optional overlay declaration; `.taskflow` gains overlay materialization metadata in the ownership/recovery contract.
- Affected documentation and guidance: `README.md`, `skills/taskflow/SKILL.md`, and the related OpenSpec capabilities.
- No new external dependency is required; Git path discovery and filesystem copying use the existing command runner and standard library.
