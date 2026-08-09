## MODIFIED Requirements

### Requirement: Aggregate task status
The CLI SHALL provide `status <task-id>` with per-repository branch, worktree, dirty-file, task phase, and active-session information in text and JSON modes, without OpenSpec change data.

#### Scenario: Status after start
- **WHEN** a task has started worktrees
- **THEN** status reports each repository's current branch and worktree facts without OpenSpec fields

### Requirement: Validate repositories and configured checks
The CLI SHALL provide `validate <task-id>` that checks configuration, Git/worktree readiness, and configured checks in dependency order, returning a failing validation result when any check fails or times out. Validation MUST NOT inspect or invoke OpenSpec.

#### Scenario: Check failure
- **WHEN** a configured check exits non-zero
- **THEN** validate records the repository/check failure and returns the validation-failure exit code

### Requirement: Produce a non-mutating finish report
`finish <task-id> --dry-run` MUST report failed validation, dirty Git state, and recommended order without changing any repository, branch, or worktree. It MUST NOT inspect OpenSpec tasks or artifacts.

#### Scenario: Finish with failed validation
- **WHEN** one repository has a failed configured check
- **THEN** finish reports the blocking repository and does not archive or clean it
