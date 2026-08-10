## MODIFIED Requirements

### Requirement: Load a strict task configuration
The CLI MUST decode `taskflow.yaml` with unknown fields rejected, apply the current internal configuration version, normalize source paths to absolute paths, derive the root from the selected task workspace, require `task.id` to match the selected task directory, preserve repository order, and use the first repository as primary. Removed fields are outside the current configuration contract and are rejected by strict decoding.

#### Scenario: Reject an unknown YAML field
- **WHEN** a task configuration contains an unrecognized field
- **THEN** task loading returns a configuration error identifying that field

#### Scenario: Reject removed development configuration
- **WHEN** a task configuration contains the removed top-level `development` field
- **THEN** loading returns a strict configuration error identifying `development`

### Requirement: Validate repository and dependency constraints
The CLI MUST require unique repository names matching the supported name pattern, an existing source directory, worktree paths contained beneath the task's `worktrees` directory, and an acyclic `depends_on` graph whose references resolve to declared repositories. Loading and structural validation MUST NOT launch external commands.

#### Scenario: Reject a dependency cycle
- **WHEN** repositories form a circular dependency through `depends_on`
- **THEN** configuration validation fails with a dependency-cycle diagnostic

#### Scenario: Reject a worktree path escape
- **WHEN** a repository worktree path resolves outside the task `worktrees` directory
- **THEN** configuration validation fails before any filesystem or Git mutation

## ADDED Requirements

### Requirement: Require current task state
Mutating and launch commands MUST require current-schema `.taskflow/state.json`. Old-schema, malformed, or wrong-task state MUST return `STATE_INCOMPATIBLE` before changing task files, Git state, worktrees, or launching a child process.

#### Scenario: Reject an old-schema workspace
- **WHEN** start, repo add, or open loads an old-schema task state
- **THEN** the command returns `STATE_INCOMPATIBLE` and requires a current task workspace
