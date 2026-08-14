# task-deletion Specification

## Purpose

Define explicit, ownership-checked cleanup of Taskflow-created task workspaces.

## Requirements

### Requirement: Delete only Taskflow-owned task resources

The CLI SHALL provide `taskflow delete <task-id> [--dry-run|--execute] [--force]`. Dry-run SHALL be the default and MUST not modify files or Git state. Execute mode MUST require a valid ownership manifest whose entries exactly match the current taskflow.yaml repositories, source paths, branches, and worktree targets.

#### Scenario: Preview an owned task deletion

- **WHEN** a task has a valid ownership manifest and the user runs delete without --execute
- **THEN** output lists worktree removal, local branch deletion, and task-directory cleanup actions without mutation

#### Scenario: Reject a task without ownership

- **WHEN** a task has no ownership manifest or its entries do not exactly match taskflow.yaml
- **THEN** delete returns a structured ownership conflict and leaves all resources unchanged

### Requirement: Preflight destructive cleanup

Execute-mode delete MUST acquire the task lock and all configured source-branch locks, verify every owned target's live Git identity, reject dirty worktrees unless --force is supplied, require a resolvable source default branch and protect it, and reject unmanaged task-directory entries before Git mutation.

#### Scenario: Reject unsafe cleanup before mutation

- **WHEN** a target is dirty, mismatched, registered at another path, uses a protected branch, or the task directory contains unmanaged content
- **THEN** delete returns a deterministic conflict and preserves the task, worktrees, branches, and files

### Requirement: Clean owned resources only

After successful preflight, delete SHALL remove only the ownership-checked worktrees, their local branches, and the empty task directory. It MUST NOT delete source repositories, default branches, remote branches, or manually managed worktrees. `--force` MAY permit dirty worktree removal and unmerged local branch deletion only when combined with --execute.

#### Scenario: Complete an owned cleanup

- **WHEN** all configured worktrees are owned, structurally matching, and safe to remove
- **THEN** delete removes the worktrees, local task branches, ownership manifest, task configuration, and empty task directory

#### Scenario: Report partial cleanup

- **WHEN** a worktree or local branch removal fails after mutation begins
- **THEN** delete returns the partial-completion exit code, reports completed and remaining actions, and preserves the remaining task metadata for retry
