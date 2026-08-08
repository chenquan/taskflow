## MODIFIED Requirements

### Requirement: Aggregate task status
The CLI SHALL provide `status <task-id>` with typed task phase and active-session data plus per-repository worktree, branch, HEAD, dirty-file count, upstream/push state, OpenSpec artifact/task progress or not-configured state, dependency readiness, and latest validation summary in text and JSON modes.

#### Scenario: Status after start
- **WHEN** a multi-repository task has started worktrees and configured changes
- **THEN** status reports each repository's current branch, HEAD, dirty-file count, change progress, dependency readiness, and active session

#### Scenario: Status without OpenSpec creation
- **WHEN** change creation is disabled
- **THEN** status reports OpenSpec as not configured rather than missing or invalid

### Requirement: Validate repositories and configured checks
The CLI SHALL provide `validate <task-id> [--repo <name>]` that checks strict OpenSpec status when configured and runs configured checks sequentially in topological dependency order. Repository-scoped validation MUST include the selected repository and its transitive dependencies. The command MUST atomically persist a report containing the normalized configuration digest, selected worktree HEADs, per-check and OpenSpec results, completion time, and overall success. Any validation failure or timeout MUST return exit code 7; malformed OpenSpec output MUST return exit code 6.

#### Scenario: Check failure
- **WHEN** a configured check exits non-zero
- **THEN** validate records the repository/check failure, persists a failed report, and returns the validation-failure exit code

#### Scenario: Validate one dependent repository
- **WHEN** a user scopes validation to a repository with transitive dependencies
- **THEN** validation runs the dependency closure in dependency-first order and excludes unrelated repositories

#### Scenario: Persist a successful fingerprint
- **WHEN** every selected OpenSpec validation and check succeeds
- **THEN** the report records the exact normalized configuration digest and worktree HEADs with overall success

### Requirement: Produce a non-mutating finish report
`finish <task-id> --dry-run` MUST NOT run configured checks or write task metadata. It MUST require a successful validation report whose configuration digest and repository HEADs match current facts, inspect current OpenSpec completion and dirty Git state read-only, report dependency-first validation/merge order and reverse archive/cleanup order, and leave every task, repository, branch, worktree, report, and OpenSpec artifact unchanged. Missing, failed, or stale validation reports, incomplete OpenSpec work, or dirty worktrees MUST block readiness with exit code 7. Unpushed branches MUST be warnings and cleanup blockers but MUST NOT block local readiness.

#### Scenario: Finish with incomplete work
- **WHEN** one repository still has incomplete tasks
- **THEN** finish reports the blocking repository and does not run checks, write reports, archive, or clean it

#### Scenario: Finish with stale validation
- **WHEN** configuration or a selected worktree HEAD differs from the last successful validation report
- **THEN** finish reports a stale-validation blocker and exits with code 7 without changing state

#### Scenario: Finish is byte-for-byte non-mutating
- **WHEN** a fresh successful report exists and finish reports readiness
- **THEN** all task metadata, report files, worktree facts, branches, and OpenSpec files remain byte-for-byte unchanged
