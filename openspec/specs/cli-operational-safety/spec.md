## Purpose

Define path containment, process streaming, managed-worktree inspection, and fetch safety.

## Requirements

### Requirement: Contain task paths
The CLI MUST reject task IDs that are not a single safe path component before resolving configuration or writing task files.

#### Scenario: Reject traversal task ID
- **WHEN** a user passes `../other-task` to init or any task-loading command
- **THEN** the command returns a configuration error and writes no file outside the tasks root

### Requirement: Run interactive tools with streams and environment
When command streams are supplied, the runner MUST execute the child without output capture conflicts and MUST apply explicit environment overlays. Non-streaming commands MUST retain captured stdout/stderr behavior.

#### Scenario: Launch Claude with instruction overlay
- **WHEN** Claude additional instructions are enabled
- **THEN** the child process receives `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1`

### Requirement: Validate and report actual managed repositories
Configuration validation MUST reject a non-Git source directory. Status MUST inspect the managed worktree for dirty state and branch.

#### Scenario: Dirty managed worktree
- **WHEN** a managed worktree has an uncommitted file while source is clean
- **THEN** status reports the managed worktree as dirty

### Requirement: Honor fetch and failure state
When `execution.fetch` is true, start dry-run MUST list and execute MUST perform a fetch before each repository worktree action.
