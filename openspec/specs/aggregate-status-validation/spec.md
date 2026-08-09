## Purpose

Define aggregate status and validation reporting.

## Requirements

### Requirement: Aggregate task status
The CLI SHALL provide `status <task-id>` with per-repository branch/worktree, dirty-file, and task phase information in text and JSON modes.

#### Scenario: Status after start
- **WHEN** a task has started worktrees and changes
- **THEN** status reports each repository's current branch and OpenSpec change presence

### Requirement: Validate repositories and configured checks
The CLI SHALL provide `validate <task-id>` that checks configuration and configured checks in dependency order, returning a failing validation result when any check fails or times out.

#### Scenario: Check failure
- **WHEN** a configured check exits non-zero
- **THEN** validate records the repository/check failure and returns the validation-failure exit code
