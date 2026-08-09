## Purpose

Define the strict task configuration contract and normalization rules.

## Requirements

### Requirement: Load a strict task configuration
The CLI MUST decode `specflow.yaml` with unknown fields rejected, apply the current internal configuration version when omitted, normalize source paths to absolute paths, derive the task root from the task workspace path, and use the first repository as primary when no primary is specified.

#### Scenario: Reject an unknown YAML field
- **WHEN** a task configuration contains an unrecognized field
- **THEN** task loading returns a configuration error identifying that field

### Requirement: Validate repository and dependency constraints
The CLI MUST require unique repository names matching the supported name pattern, an existing source directory, a primary repository present in the repository set, worktree paths contained beneath the task's `worktrees` directory, and an acyclic `depends_on` graph whose references resolve to declared repositories. Loading and structural validation MUST NOT launch external commands.

#### Scenario: Reject a dependency cycle
- **WHEN** repositories form a circular dependency through `depends_on`
- **THEN** configuration validation fails with a dependency-cycle diagnostic

#### Scenario: Reject a worktree path escape
- **WHEN** a repository worktree path resolves outside the task `worktrees` directory
- **THEN** configuration validation fails before any filesystem or Git mutation
