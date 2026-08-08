## Purpose

Define task completeness, source integrity, and mutation-free initialization rejection.

## Requirements

### Requirement: Block readiness on incomplete OpenSpec tasks
Validation and `finish --dry-run` SHALL treat a configured OpenSpec change as incomplete when its `tasks.md` is missing, unreadable, or contains an unchecked Markdown task checkbox. The result MUST identify the affected repository and return the validation-failure exit code.

#### Scenario: Change has unchecked task
- **WHEN** a managed change contains a `tasks.md` file with an unchecked task
- **THEN** validation and the finish readiness report identify that repository as blocked and exit with code 7

### Requirement: Require a non-bare Git worktree source
Configuration validation SHALL accept a repository source only when `git rev-parse --is-inside-work-tree` reports `true`.

#### Scenario: Bare repository source
- **WHEN** a configured source is an existing bare Git repository
- **THEN** configuration validation rejects it before start or doctor is run

### Requirement: Reject invalid initialization without a task directory
`init` SHALL validate its fully constructed configuration before creating the final task directory.

#### Scenario: Unknown primary repository
- **WHEN** an init request names a primary repository not declared by `--repo`
- **THEN** init returns a configuration error and does not create `<tasks-root>/<task-id>`
