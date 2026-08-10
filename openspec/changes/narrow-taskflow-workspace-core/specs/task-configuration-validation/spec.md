## MODIFIED Requirements

### Requirement: Load a strict task configuration
The CLI MUST decode `taskflow.yaml` with unknown fields rejected, apply the current internal configuration version, normalize source paths to absolute paths, derive the root from the selected task workspace, require `task.id` to match the selected task directory, preserve repository order, and use the first repository as primary. A top-level legacy `development` field MUST return `LEGACY_CONFIGURATION_UNSUPPORTED` with reinitialization guidance before any mutation.

#### Scenario: Reject an unknown YAML field
- **WHEN** a task configuration contains an unrecognized field other than the recognized legacy development field
- **THEN** task loading returns a configuration error identifying that field

#### Scenario: Reject legacy development configuration
- **WHEN** a task configuration contains the removed top-level `development` field
- **THEN** loading returns `LEGACY_CONFIGURATION_UNSUPPORTED`, preserves every workspace file, and advises reinitialization in an empty task directory

### Requirement: Validate repository and dependency constraints
The CLI MUST require unique repository names matching the supported name pattern, an existing source directory, worktree paths contained beneath the task's `worktrees` directory, and an acyclic `depends_on` graph whose references resolve to declared repositories. Loading and structural validation MUST NOT launch external commands.

#### Scenario: Reject a dependency cycle
- **WHEN** repositories form a circular dependency through `depends_on`
- **THEN** configuration validation fails with a dependency-cycle diagnostic

#### Scenario: Reject a worktree path escape
- **WHEN** a repository worktree path resolves outside the task `worktrees` directory
- **THEN** configuration validation fails before any filesystem or Git mutation

## ADDED Requirements

### Requirement: Reject legacy task state without mutation
Mutating and launch commands MUST require schema-v2 `.taskflow/state.json`. Schema-v1, malformed, or wrong-task state MUST return `STATE_INCOMPATIBLE` before changing task files, Git state, worktrees, or launching a child process.

#### Scenario: Reject a schema-v1 workspace
- **WHEN** start, repo add, or open loads schema-v1 task state
- **THEN** the command returns `STATE_INCOMPATIBLE`, preserves the existing workspace, and advises reinitialization
