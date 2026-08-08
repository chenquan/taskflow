## ADDED Requirements

### Requirement: Load a strict versioned task configuration
The CLI MUST decode `specflow.yaml` with unknown fields rejected and MUST reject unsupported configuration versions. It MUST normalize source paths to absolute paths and preserve task identity separately from derived branch or change names.

#### Scenario: Reject an unknown YAML field
- **WHEN** a task configuration contains an unrecognized field
- **THEN** `specflow config validate` returns a configuration error identifying that field

### Requirement: Validate repository and dependency constraints
The CLI MUST require unique repository names matching the supported name pattern, an existing Git source directory, a primary repository present in the repository set, worktree paths contained beneath the task's `worktrees` directory, and an acyclic `depends_on` graph whose references resolve to declared repositories.

#### Scenario: Reject a dependency cycle
- **WHEN** repositories form a circular dependency through `depends_on`
- **THEN** configuration validation fails with a dependency-cycle diagnostic

#### Scenario: Reject a worktree path escape
- **WHEN** a repository worktree path resolves outside the task `worktrees` directory
- **THEN** configuration validation fails before any filesystem or Git mutation

### Requirement: Display normalized configuration
The CLI SHALL provide `specflow config show <task-id>` and `specflow config validate <task-id>` using the same validated configuration model. The show command MUST not mutate the task workspace or source repositories.

#### Scenario: Show a valid configuration as JSON
- **WHEN** a user requests `config show` with JSON output for a valid task
- **THEN** the response includes the normalized configuration in the stable result envelope
