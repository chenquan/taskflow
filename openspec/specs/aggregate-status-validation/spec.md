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

### Requirement: Report stale validation results
Status MUST compare the persisted validation report's configuration digest against the current normalized task configuration. When the report digest no longer matches, status MUST report `validationStale: true`, MUST NOT present the stale report as the current `lastValidation` result, and MUST NOT attribute the stale per-repository validation outcomes to the current repositories. The command MUST leave the persisted `validation.json` on disk as historical evidence, and the next `validate` MUST regenerate a report whose digest matches the current configuration.

#### Scenario: Status flags a stale report after append
- **WHEN** a task has a persisted validation report and its configuration changes because `repo add` advanced the digest
- **THEN** status reports `validationStale: true`, omits the stale per-repository validation outcomes, and leaves `validation.json` on disk

#### Scenario: Validate refreshes the report after append
- **WHEN** the user runs `validate` after the append advanced the digest
- **THEN** the regenerated report's digest matches the current configuration and status no longer reports a stale report
