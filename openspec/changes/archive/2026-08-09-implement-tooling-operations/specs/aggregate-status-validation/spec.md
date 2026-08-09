## ADDED Requirements

### Requirement: Aggregate task status
The CLI SHALL provide `status <task-id>` with per-repository branch/worktree, dirty-file, OpenSpec change, task phase, and active-session information in text and JSON modes.

#### Scenario: Status after start
- **WHEN** a task has started worktrees and changes
- **THEN** status reports each repository's current branch and OpenSpec change presence

### Requirement: Validate repositories and configured checks
The CLI SHALL provide `validate <task-id>` that checks configuration, OpenSpec change status, and configured checks in dependency order, returning a failing validation result when any check fails or times out.

#### Scenario: Check failure
- **WHEN** a configured check exits non-zero
- **THEN** validate records the repository/check failure and returns the validation-failure exit code

### Requirement: Produce a non-mutating finish report
`finish <task-id> --dry-run` MUST report incomplete OpenSpec tasks, failed validation, dirty Git state, and recommended order without changing any repository, branch, worktree, or OpenSpec artifact.

#### Scenario: Finish with incomplete work
- **WHEN** one repository still has incomplete tasks
- **THEN** finish reports the blocking repository and does not archive or clean it
