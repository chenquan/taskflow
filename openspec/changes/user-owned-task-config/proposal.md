## Why

Taskflow is used by AI agents as a small worktree preparation and CLI launch tool, but its current `create --repo` behavior also acts as an incremental repository configuration manager. That imperative append path makes `taskflow.yaml` less clearly user-owned and keeps extra reconciliation and persistence branches in the core command.

## What Changes

- **BREAKING** Keep repeated `create --repo <name>=<path>` declarations for bootstrapping a task that has no `taskflow.yaml`.
- **BREAKING** Reject `create --repo` when `taskflow.yaml` already exists; users and AI agents must edit the desired configuration directly and rerun `create` without repository arguments.
- Treat `taskflow.yaml` as user/agent-owned desired configuration and avoid rewriting it during reconciliation of an existing task.
- Preserve live Git preflight, task/source locks, idempotent create/reuse behavior, partial-failure retry, and safe non-destructive conflict handling.
- Update the README, bundled Taskflow skill, active specifications, unit tests, and Git-backed E2E tests for the new configuration ownership boundary.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `worktree-reconciliation`: restricts `--repo` to first-time configuration bootstrap and adds direct-config reconciliation scenarios.
- `task-workspace-initialization`: removes existing-task append semantics while retaining safe initial declaration and atomic bootstrap persistence.
- `e2e-command-flow`: replaces append-through-create coverage with user/agent-edited configuration coverage.
- `taskflow-multirepo-skill`: instructs agents to edit `taskflow.yaml` for topology changes and then run create dry-run/execute.
- `repository-append`: retires the active append capability; historical archived append specifications remain unchanged.

## Impact

The Cobra help and `Create` application service will change, including a new machine-readable `CONFIG_EDIT_REQUIRED` diagnostic for repository arguments on an existing task. Documentation, bundled AI guidance, active OpenSpec requirements, and app/CLI/E2E tests will be updated. Existing task directories and archived historical OpenSpec artifacts are not migrated or rewritten.
