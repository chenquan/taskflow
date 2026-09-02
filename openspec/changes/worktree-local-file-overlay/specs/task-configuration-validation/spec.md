## MODIFIED Requirements

### Requirement: Load a strict task configuration
The CLI MUST decode taskflow.yaml with unknown fields rejected, apply the current internal configuration version when omitted, normalize source paths to absolute paths, derive the task root from the selected task workspace, require task.id to match the selected task directory, preserve repository order, and accept an optional `local.paths` overlay block for each repository. State, inventory, validation, execution, dependency, check, and unrecognized overlay fields are outside the current configuration contract and MUST be rejected by strict decoding.

#### Scenario: Reject an unknown YAML field
- **WHEN** a taskflow.yaml contains an unrecognized or retired field
- **THEN** task loading returns a configuration error identifying that field

#### Scenario: Preserve repository order
- **WHEN** taskflow.yaml lists repositories in a specific order
- **THEN** loading preserves that order and the first repository remains primary

#### Scenario: Load an optional local overlay
- **WHEN** a repository contains a valid `local.paths` list
- **THEN** configuration loading preserves the declared order and normalizes each path relative to the repository source without invoking create or copying files

### Requirement: Validate repository and dependency constraints
The CLI MUST require unique repository names matching the supported name pattern, existing source directories, non-empty base and branch values, worktree paths contained beneath the task worktrees directory, unique target paths, and overlay paths that are non-empty, exact, source-relative, and free of path escapes or `.git` components. Loading and structural validation MUST NOT launch external commands or mutate the filesystem; Git tracked-state, file-type, and base-tree checks belong to create preflight.

#### Scenario: Reject a worktree path escape
- **WHEN** a repository worktree path resolves outside the task worktrees directory
- **THEN** task loading fails before any filesystem or Git mutation

#### Scenario: Reject duplicate targets
- **WHEN** two repositories resolve to the same worktree target
- **THEN** configuration validation returns a conflict before create

#### Scenario: Reject an absolute or escaping overlay path
- **WHEN** `local.paths` contains an absolute path or a path containing traversal outside the source
- **THEN** configuration validation returns an error before create preflight or mutation

#### Scenario: Reject duplicate overlay paths
- **WHEN** one repository declares the same local overlay path more than once after normalization
- **THEN** configuration validation returns a deterministic duplicate-path error
