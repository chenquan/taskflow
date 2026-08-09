## Purpose

Define task completeness, source integrity, and mutation-free initialization rejection.

## Requirements

### Requirement: Require a non-bare Git worktree source
Initialization and execute-mode preflight SHALL accept a repository source only when Git inspection reports a non-bare worktree.

#### Scenario: Bare repository source
- **WHEN** a configured source is an existing bare Git repository
- **THEN** initialization rejects it before creating a task workspace

### Requirement: Reject invalid initialization without a task directory
`init` SHALL validate its fully constructed configuration before creating the final task directory.

#### Scenario: Unknown primary repository
- **WHEN** an init request names a primary repository not declared by `--repo`
- **THEN** init returns a configuration error and does not create `<tasks-root>/<task-id>`
