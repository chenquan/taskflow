## Purpose

Define the strict task configuration contract and normalization rules.

## Requirements

### Requirement: Load a strict versioned task configuration
The CLI MUST decode `specflow.yaml` with unknown fields rejected and MUST reject unsupported configuration versions. It MUST normalize source paths to absolute paths and preserve task identity separately from derived branch or change names.

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
