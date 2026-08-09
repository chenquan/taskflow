## Purpose

Define aggregate status, dependency-aware validation, and persisted validation reporting for managed task worktrees.

## Requirements

### Requirement: Aggregate task status
The CLI SHALL provide `status <task-id>` with per-repository branch/worktree, dirty-file, and task phase information in text and JSON modes.

#### Scenario: Status after start
- **WHEN** a task has started managed worktrees
- **THEN** status reports each repository's current branch, worktree, dirty state, upstream synchronization, dependency readiness, and latest validation result when available

### Requirement: Validate repositories and configured checks
The CLI SHALL provide `validate <task-id>` that inspects managed worktrees and runs configured checks in dependency order, returning a failing validation result when any inspection or check fails or times out. `validate <task-id> --repo <repo-name>` SHALL validate that repository together with its dependency closure.

#### Scenario: Check failure
- **WHEN** a configured check exits non-zero
- **THEN** validate records the repository/check failure and returns the validation-failure exit code
