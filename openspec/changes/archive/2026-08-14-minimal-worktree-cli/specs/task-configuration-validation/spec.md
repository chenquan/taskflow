## MODIFIED Requirements

### Requirement: Load a strict task configuration
The CLI MUST decode taskflow.yaml with unknown fields rejected, apply the current internal configuration version when omitted, normalize source paths to absolute paths, derive the task root from the selected task workspace, require task.id to match the selected task directory, preserve repository order, and use the first repository as primary. State, inventory, validation, execution, dependency, and check fields are outside the current configuration contract and MUST be rejected by strict decoding.

#### Scenario: Reject an unknown YAML field
- **WHEN** a taskflow.yaml contains an unrecognized or retired field
- **THEN** task loading returns a configuration error identifying that field

#### Scenario: Preserve repository order
- **WHEN** taskflow.yaml lists repositories in a specific order
- **THEN** loading preserves that order and open uses the first repository as cwd

### Requirement: Validate repository and dependency constraints
The CLI MUST require unique repository names matching the supported name pattern, existing source directories, non-empty base and branch values, worktree paths contained beneath the task worktrees directory, and unique target paths. Loading and structural validation MUST NOT launch external commands.

#### Scenario: Reject a worktree path escape
- **WHEN** a repository worktree path resolves outside the task worktrees directory
- **THEN** task loading fails before any filesystem or Git mutation

#### Scenario: Reject duplicate targets
- **WHEN** two repositories resolve to the same worktree target
- **THEN** configuration validation returns a conflict before create or open
